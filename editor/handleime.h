#pragma once

#ifndef GO_HANDLEIME_H
#define GO_HANDLEIME_H

#include <stdint.h>
#include <stdio.h>

#ifdef __cplusplus
extern "C" {
#endif

int selectionLengthInPreeditStr(void* ptr, int cursorpos);
int selectionLengthInPreeditStrOnDarwin(void* ptr, int cursorpos);
int cursorPosInPreeditStr(void* ptr);

#ifdef __cplusplus
}
#endif

#endif
