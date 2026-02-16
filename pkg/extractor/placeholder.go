package extractor

import (
	"fmt"

	"github.com/l3aro/go-context-query/pkg/types"
	sitter "github.com/smacker/go-tree-sitter"
)

type notImplementedExtractor struct {
	lang Language
}

func (e *notImplementedExtractor) Extract(file string) (*types.ModuleInfo, error) {
	return nil, fmt.Errorf("%s extractor not yet implemented", e.lang)
}

func (e *notImplementedExtractor) Language() Language {
	return e.lang
}

func (e *notImplementedExtractor) FileExtensions() []string {
	return nil
}

func NewSwiftExtractor() Extractor   { return &notImplementedExtractor{Swift} }
func NewSwiftParser() *sitter.Parser { return nil }
