//go:build darwin && cgo

package keyboard

/*
#cgo LDFLAGS: -framework Carbon -framework CoreFoundation
#include <Carbon/Carbon.h>
#include <CoreFoundation/CoreFoundation.h>

const char* qedit_current_keyboard_layout(void) {
    // Process pending input source change notifications
    CFRunLoopRunInMode(kCFRunLoopDefaultMode, 0, false);

    TISInputSourceRef source = TISCopyCurrentKeyboardInputSource();
    if (!source) return NULL;
    CFStringRef prop = TISGetInputSourceProperty(source, kTISPropertyInputModeID);
    if (!prop) {
        prop = TISGetInputSourceProperty(source, kTISPropertyInputSourceID);
    }
    if (!prop) {
        prop = TISGetInputSourceProperty(source, kTISPropertyLocalizedName);
    }
    if (!prop) {
        CFRelease(source);
        return NULL;
    }
    static char buf[256];
    Boolean ok = CFStringGetCString(prop, buf, sizeof(buf), kCFStringEncodingUTF8);
    CFRelease(source);
    if (!ok) return NULL;
    return buf;
}
*/
import "C"

import "strings"

func CurrentLayoutRaw() string {
	cstr := C.qedit_current_keyboard_layout()
	if cstr == nil {
		return ""
	}
	return strings.TrimSpace(C.GoString(cstr))
}

var layoutAbbreviations = map[string]string{
	"ABC":       "US",
	"US":        "US",
	"Russian":   "RU",
	"RussianPC": "RU",
}

func simplifyLayoutName(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	sep := strings.LastIndexAny(raw, ".")
	if sep >= 0 && sep < len(raw)-1 {
		raw = raw[sep+1:]
	}
	raw = strings.TrimPrefix(raw, "com.apple.")
	raw = strings.TrimPrefix(raw, "keylayout.")
	raw = strings.TrimPrefix(raw, "inputmethod.")
	raw = strings.TrimPrefix(raw, "inputsource.")
	raw = strings.ReplaceAll(raw, ".", " ")
	raw = strings.ReplaceAll(raw, "-", " ")
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	// Convert to short abbreviation if known
	if abbr, ok := layoutAbbreviations[raw]; ok {
		return abbr
	}
	return raw
}

func CurrentLayout() string {
	raw := CurrentLayoutRaw()
	if raw == "" {
		return ""
	}
	return simplifyLayoutName(raw)
}
