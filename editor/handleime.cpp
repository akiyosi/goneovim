#include "handleime.h"

#include <QInputMethodEvent>


int selectionLengthInPreeditStrOnDarwin(void* ptr, int cursorpos) {

    QInputMethodEvent* event = static_cast<QInputMethodEvent*>(ptr);

    QList<QInputMethodEvent::Attribute> attributes;
    attributes = {};
    attributes = event->attributes();
    int ret = attributes.size();

    for (int i = 0; i < attributes.size(); i++) {

	const QInputMethodEvent::Attribute &a = event->attributes().at(i);

	if (a.type == QInputMethodEvent::TextFormat) {

	    if (a.start + a.length == cursorpos) {
		ret = a.length;
	    }

	}

    }

    return ret;
}

int selectionLengthInPreeditStr(void* ptr, int cursorpos) {

    QInputMethodEvent* event = static_cast<QInputMethodEvent*>(ptr);

    QList<QInputMethodEvent::Attribute> attributes;
    attributes = {};
    attributes = event->attributes();
    int ret = attributes.size();

    for (int i = 0; i < attributes.size(); i++) {

	const QInputMethodEvent::Attribute &a = event->attributes().at(i);

	if (a.type == QInputMethodEvent::TextFormat) {

	    if (a.start == cursorpos) {
		ret = a.length;
	    }

	}

    }

    return ret;
}

int cursorPosInPreeditStr(void* ptr) {

    QInputMethodEvent* event = static_cast<QInputMethodEvent*>(ptr);


    QList<QInputMethodEvent::Attribute> attributes;
    attributes = {};
    attributes = event->attributes();
    int ret = attributes.size();

    for (int i = 0; i < attributes.size(); i++) {

	const QInputMethodEvent::Attribute &a = event->attributes().at(i);

	if (a.type == QInputMethodEvent::Cursor) {

	    ret = a.start;

	}

    }

    return ret;

}
