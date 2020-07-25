package fuzzy

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	gonvimUtil "github.com/akiyosi/goneovim/util"
	"github.com/denormal/go-gitignore"
	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
	"github.com/neovim/go-client/nvim"
)

const (
	slab16Size int = 100 * 1024 // 200KB * 32 = 12.8MB
	slab32Size int = 2048       // 8KB * 32 = 256KB
)

// Fuzzy is
type Fuzzy struct {
	nvim               *nvim.Nvim
	options            map[string]interface{}
	source             []string
	sourceNew          chan string
	max                int
	selected           int
	pattern            string
	cursor             int
	slab               *util.Slab
	start              int
	result             []*Output
	scoreMutext        *sync.Mutex
	handleMutex        sync.RWMutex
	scoreNew           bool
	cancelled          bool
	cancelChan         chan bool
	lastOutput         []string
	lastMatch          [][]int
	resultRWMtext      sync.RWMutex
	running            bool
	pwd                string
	isRemoteAttachment bool
}

// Output is
type Output struct {
	result algo.Result
	match  *[]int
	output string
}

// RegisterPlugin registers this remote plugin
func RegisterPlugin(nvim *nvim.Nvim, isRemoteAttachment bool) {
	nvim.Subscribe("GonvimFuzzy")
	shim := &Fuzzy{
		nvim:               nvim,
		slab:               util.MakeSlab(slab16Size, slab32Size),
		scoreMutext:        &sync.Mutex{},
		max:                20,
		isRemoteAttachment: isRemoteAttachment,
	}
	nvim.RegisterHandler("GonvimFuzzy", func(args ...interface{}) {
		shim.handleMutex.RLock()
		go func() {
			shim.handle(args...)
			defer shim.handleMutex.RUnlock()
		}()
	})
}

// UpdateMax updates the max
func UpdateMax(nvim *nvim.Nvim, max int) {
	go nvim.Call("rpcnotify", nil, 0, "GonvimFuzzy", "update_max", max)
}

func (s *Fuzzy) handle(args ...interface{}) {
	if len(args) < 1 {
		return
	}
	event, ok := args[0].(string)
	if !ok {
		return
	}
	switch event {
	case "run":
		s.run(args[1:])
	case "char":
		s.newChar(args[1:])
	case "backspace":
		s.backspace()
	case "clear":
		s.clear()
	case "left":
		s.left()
	case "right":
		s.right()
	case "down":
		s.down()
	case "up":
		s.up()
	case "cancel":
		s.cancel()
	case "confirm":
		s.confirm()
	case "resume":
		s.resume()
	case "update_max":
		s.resultRWMtext.Lock()
		s.max = gonvimUtil.ReflectToInt(args[1])
		s.resultRWMtext.Unlock()
	default:
		fmt.Println("unhandleld fzfshim event", event)
	}
}

func (s *Fuzzy) run(args []interface{}) {
	ok := s.parseOptions(args)
	if !ok {
		return
	}
	s.running = true
	s.reset()
	s.processSource()
	s.outputPattern()
	s.filter()
}

func (s *Fuzzy) reset() {
	s.source = []string{}
	s.selected = 0
	s.pattern = ""
	s.cursor = 0
	s.start = 0
	s.pwd = ""
	s.lastOutput = []string{}
	s.lastMatch = [][]int{}
	s.sourceNew = make(chan string, 1000)
	s.cancelled = false
	s.cancelChan = make(chan bool, 1)
}

// ByScore sorts the output by score
type ByScore []*Output

func (a ByScore) Len() int {
	return len(a)
}

func (a ByScore) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a ByScore) Less(i, j int) bool {
	iout := a[i]
	jout := a[j]
	return (iout.result.Score < jout.result.Score)
}

func (s *Fuzzy) filter() {
	if s.cancelled {
		return
	}
	s.scoreNew = true
	s.scoreMutext.Lock()
	defer s.scoreMutext.Unlock()
	s.scoreNew = false
	s.resultRWMtext.Lock()
	s.result = []*Output{}
	s.resultRWMtext.Unlock()
	sourceNew := s.sourceNew

	stop := make(chan bool, 1)
	go func() {
		tick := time.Tick(50 * time.Millisecond)
		for {
			select {
			case <-tick:
				s.outputResult()
			case <-stop:
				return
			}
		}
	}()
	defer func() {
		stop <- true
	}()

	for _, source := range s.source {
		s.scoreSource(source)
		if s.scoreNew || s.cancelled {
			return
		}
	}

loop:
	for {
		select {
		case source, ok := <-sourceNew:
			if !ok {
				break loop
			}
			s.source = append(s.source, source)
			s.scoreSource(source)
			if s.scoreNew || s.cancelled {
				return
			}
		case <-time.After(1000 * time.Millisecond):
			fmt.Println("timeout reading sourceNew")
			break loop
		}
	}
	if s.scoreNew || s.cancelled {
		return
	}
	s.outputResult()
}

func (s *Fuzzy) scoreSource(source string) {
	r := algo.Result{
		Score: -1,
	}
	n := &[]int{}

	if s.pattern != "" {
		var chars util.Chars
		var parts []string

		// if fuzzy type is "file_line",
		// then exclude file name string from source string
		indexOffset := 0
		switch s.options["type"] {
		case "file_line":
			parts = strings.SplitN(source, ":", 4)
			filecontents := parts[len(parts)-1]
			chars = util.ToChars([]byte(filecontents))
			indexOffset = len(parts[0]) + 1 + len(parts[1]) + 1 + len(parts[2]) + 1
		case "line":
			parts = strings.SplitN(source, "\t", 2)
			filecontents := parts[len(parts)-1]
			chars = util.ToChars([]byte(filecontents))
			indexOffset = len(parts[0]) + 1
		default:
			chars = util.ToChars([]byte(source))
		}

		// fuzzy match like smart case
		if strings.ContainsAny(s.pattern, "ABCDEFZHIJKLMNOPQRSTUVWXYZ") {
			r, n = algo.FuzzyMatchV1(true, true, true, &chars, []rune(s.pattern), true, s.slab)
		} else {
			r, n = algo.FuzzyMatchV1(false, true, true, &chars, []rune(s.pattern), true, s.slab)
		}

		// Since the file name is excluded from the source string,
		// the number of characters of the file name is added to the index.
		if n != nil && indexOffset != 0 {
			var newN []int
			for _, idx := range *n {
				newN = append(
					newN,
					idx+indexOffset,
				)
			}
			n = &newN
		}
	}
	if r.Score == -1 || r.Score > 0 {
		i := 0
		if r.Score > 0 {
			for i = 0; i < len(s.result); i++ {
				if s.result[i].result.Score < r.Score {
					break
				}
			}
		} else {
			i = len(s.result)
		}

		s.result = append(s.result[:i],
			append(
				[]*Output{&Output{
					result: r,
					output: source,
					match:  n,
				}},
				s.result[i:]...,
			)...,
		)

		// if s.start <= i && i <= s.start+s.max-1 {
		// 	s.outputResult()
		// }
	}
}

func (s *Fuzzy) processSource() {
	source := s.options["source"]
	pwd, ok := s.options["pwd"]
	if ok {
		s.pwd, ok = pwd.(string)
		if !ok {
			s.pwd = ""
		}
		path, err := gonvimUtil.ExpandTildeToHomeDirectory(s.pwd)
		if err == nil {
			s.pwd = path
		}
	}
	if s.pwd != "" {
		os.Chdir(s.pwd)
	}
	sourceNew := s.sourceNew
	cancelChan := s.cancelChan
	if source == nil {
		dir := ""
		dirInterface, ok := s.options["dir"]
		if ok {
			dir, ok = dirInterface.(string)
			if !ok {
				dir = ""
			}
			path, err := gonvimUtil.ExpandTildeToHomeDirectory(dir)
			if err == nil {
				dir = path
			}
		}
		homeDir := ""
		usr, err := user.Current()
		if err == nil {
			homeDir = usr.HomeDir
		}
		go func() {
			defer close(sourceNew)
			pwd := "./"
			if dir != "" {
				pwd = dir
			}

			if !s.isRemoteAttachment {
				files, _ := ioutil.ReadDir(pwd)
				folders := []string{}
				ignore, _ := gitignore.NewRepository(pwd)
				for {
					for _, f := range files {
						if s.cancelled {
							return
						}
						if f.IsDir() {
							if f.Name() == ".git" {
								continue
							}
							folders = append(folders, filepath.Join(pwd, f.Name()))
							continue
						}
						file := filepath.Join(pwd, f.Name())
						match := ignore.Relative(file, true)
						if match != nil {
							continue
						}
						if homeDir != "" && strings.HasPrefix(file, homeDir) {
							file = "~" + file[len(homeDir):]
						}
						select {
						case sourceNew <- file:
						case <-cancelChan:
							return
						}
					}
					for {
						if len(folders) == 0 {
							return
						}
						pwd = folders[0]
						folders = folders[1:]
						files, _ = ioutil.ReadDir(pwd)
						if len(files) == 0 {
							continue
						} else {
							break
						}
					}
				}
			} else {
				// -- Explore file with nvim function
				// --
				folders := []string{}
				ignore, _ := gitignore.NewRepository(pwd)
				command := fmt.Sprintf("globpath('%s', '{,.}*', 1, 0)", pwd)
				files := ""
				s.nvim.Eval(command, &files)
				for {
					for _, file := range strings.Split(files, "\n") {
						if s.cancelled {
							return
						}

						file = file[2:]

						// Skip './' and '../'
						if file[len(file)-2:] == "./" || file[len(file)-3:] == "../" {
							continue
						}

						// If it is directory
						if file[len(file)-1] == '/' {
							if file == ".git/" {
								continue
							}
							folders = append(folders, file)
							continue
						}

						// Skip gitignore files
						if !s.isRemoteAttachment {
							match := ignore.Relative(file, true)
							if match != nil {
								fmt.Println("ignore!")
								continue
							}
						}

						select {
						case sourceNew <- file:
						case <-cancelChan:
							return
						}
					}
					for {
						if len(folders) == 0 {
							return
						}
						command = fmt.Sprintf("globpath('./%s', '{,.}*', 1, 0)", folders[0])
						folders = folders[1:]
						s.nvim.Eval(command, &files)
						if len(files) == 0 {
							continue
						} else {
							break
						}
					}
				}
			}
		}()
		return
	}
	switch src := source.(type) {
	case []interface{}:
		go func() {
			for _, item := range src {
				if s.cancelled {
					close(sourceNew)
					return
				}
				str, ok := item.(string)
				if !ok {
					continue
				}

				select {
				case sourceNew <- str:
				case <-cancelChan:
					close(sourceNew)
					return
				}
			}
			close(sourceNew)
		}()
	case string:
		cmd := exec.Command("bash", "-c", src)
		gonvimUtil.PrepareRunProc(cmd)
		stdout, _ := cmd.StdoutPipe()
		output := ""
		go func() {
			buf := make([]byte, 2)
			for {
				n, err := stdout.Read(buf)
				if err != nil || s.cancelled {
					close(sourceNew)
					stdout.Close()
					cmd.Wait()
					return
				}
				output += string(buf[0:n])
				parts := strings.Split(output, "\n")
				if len(parts) > 1 {
					for i := 0; i < len(parts)-1; i++ {
						// s.source = append(s.source, parts[i])
						select {
						case sourceNew <- parts[i]:
						case <-cancelChan:
							close(sourceNew)
							stdout.Close()
							cmd.Wait()
							return
						}
					}
					output = parts[len(parts)-1]
				}
			}
		}()
		cmd.Start()
	default:
		fmt.Println(reflect.TypeOf(source))
	}
}

func (s *Fuzzy) parseOptions(args []interface{}) bool {
	if len(args) == 0 {
		return false
	}
	options, ok := args[0].(map[string]interface{})
	if !ok {
		return false
	}
	s.options = options
	return true
}

func (s *Fuzzy) newChar(args []interface{}) {
	if len(args) == 0 {
		return
	}
	c, ok := args[0].(string)
	if !ok {
		return
	}
	if len(c) == 0 {
		return
	}
	s.pattern = insertAtIndex(s.pattern, s.cursor, c)
	s.cursor++
	s.outputPattern()
	s.filter()
}

func (s *Fuzzy) clear() {
	s.pattern = ""
	s.cursor = 0
	s.outputPattern()
	s.filter()
}

func (s *Fuzzy) backspace() {
	if s.cursor == 0 {
		return
	}
	s.cursor--
	s.pattern = removeAtIndex(s.pattern, s.cursor)
	s.outputPattern()
	s.filter()
}

func (s *Fuzzy) left() {
	if s.cursor > 0 {
		s.cursor--
	}
	s.outputCursor()
}

func (s *Fuzzy) outputPattern() {
	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_pattern", s.pattern, s.cursor)
}

func (s *Fuzzy) outputHide() {
	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_hide")
}

func (s *Fuzzy) outputShow() {
	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_show")
}

func (s *Fuzzy) outputCursor() {
	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_pattern_pos", s.cursor)
}

func outputEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func matchIntSliceEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func matchEqual(a, b [][]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !matchIntSliceEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func (s *Fuzzy) outputResult() {
	if !s.running {
		return
	}
	start := s.start
	result := s.result
	selected := s.selected
	s.resultRWMtext.RLock()
	max := s.max
	total := len(result)
	if start >= total || selected >= total {
		s.start = 0
		start = 0
		s.selected = 0
	}
	end := start + max
	if end > total {
		end = total
	}
	output := []string{}
	match := [][]int{}
	for _, o := range result[start:end] {
		text := o.output
		if len(text) > 200 {
			text = text[:200]
		}
		output = append(output, text)
	}
	for _, o := range result[start:end] {
		if o.match == nil {
			match = append(match, []int{})
		} else {
			match = append(match, *o.match)
		}
	}
	s.resultRWMtext.RUnlock()

	if outputEqual(output, s.lastOutput) && matchEqual(match, s.lastMatch) {
		return
	}
	s.lastOutput = output
	s.lastMatch = match

	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_show_result", output, selected-start, match, s.options["type"], start, total)
}

func (s *Fuzzy) right() {
	if s.cursor < len(s.pattern) {
		s.cursor++
	}
	s.outputCursor()
}

func (s *Fuzzy) up() {
	if s.selected > 0 {
		s.selected--
	} else if s.selected == 0 {
		s.selected = len(s.result) - 1
	}
	s.processSelected()
}

func (s *Fuzzy) down() {
	if s.selected < len(s.result)-1 {
		s.selected++
	} else if s.selected == len(s.result)-1 {
		s.selected = 0
	}
	s.processSelected()
}

func (s *Fuzzy) processSelected() {
	s.resultRWMtext.RLock()
	if s.selected < s.start {
		s.start = s.selected
		s.outputResult()
	} else if s.selected >= s.start+s.max {
		s.start = s.selected - s.max + 1
		s.outputResult()
	}
	s.resultRWMtext.RUnlock()
	go s.nvim.Call("rpcnotify", nil, 0, "Gui", "finder_select", s.selected-s.start)
}

func (s *Fuzzy) confirm() {
	if s.selected >= len(s.result) {
		s.cancel()
		return
	}
	arg := s.result[s.selected].output
	s.cancel()

	sink, ok := s.options["sink"]
	if ok {
		go s.nvim.Command(fmt.Sprintf("%s %s", sink.(string), arg))
		return
	}

	function, ok := s.options["function"]
	if ok {
		options := map[string]string{}
		options["function"] = function.(string)
		options["arg"] = arg
		go s.nvim.Call("gonvim_fuzzy#exec", nil, options)
	}
}

func (s *Fuzzy) cancel() {
	s.running = false
	s.outputHide()
	// s.cancelled = true
	// s.cancelChan <- true
	// s.reset()
}

func (s *Fuzzy) resume() {
	s.running = true
	s.outputShow()
	// s.cancelled = false
	// s.cancelChan <- false
}

func removeAtIndex(in string, i int) string {
	if len(in) == 0 {
		return in
	}
	if i >= len(in) {
		return in
	}
	a := []rune(in)
	a = append(a[:i], a[i+1:]...)
	return string(a)
}

func insertAtIndex(in string, i int, newChar string) string {
	a := []rune(in)
	a = append(a[:i], append([]rune{rune(newChar[0])}, a[i:]...)...)
	return string(a)
}

// func reflectToInt(iface interface{}) int {
// 	o, ok := iface.(int64)
// 	if ok {
// 		return int(o)
// 	}
// 	return int(iface.(uint64))
// }

