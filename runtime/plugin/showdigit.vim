" map 0-9 to nth item
let s:shiftKeys = '!@#$%^&*('
for s:i in range(0, 9)
  exe printf('inoremap <expr> %d pumvisible() ? <sid>select_pum(%d) : %d ', s:i, s:i, s:i)
endfor

function! s:select_pum(index)
  let compInfo = complete_info()
  let idx = a:index == 0 ? 10 : a:index - 1
  let d = idx - compInfo.selected
  return repeat( d > 0 ? "\<c-n>" : "\<c-p>", abs(d)) . "\<C-y>"
endfunction
