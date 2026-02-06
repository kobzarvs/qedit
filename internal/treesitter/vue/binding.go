package vue

//#include "tree_sitter/parser.h"
//TSLanguage *tree_sitter_vue();
import "C"
import (
	"unsafe"

	sitter "github.com/smacker/go-tree-sitter"
)

func GetLanguage() *sitter.Language {
	ptr := unsafe.Pointer(C.tree_sitter_vue())
	return sitter.NewLanguage(ptr)
}
