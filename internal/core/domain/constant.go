package domain

import (
	"regexp"
	"sync"
)

type PIIMasker struct {
	patterns    map[string]*MaskingPattern
	fieldRules  map[string]MaskingRule
	globalRules []GlobalMaskingRule
	mu          sync.RWMutex
	enabled     bool
}

type MaskingPattern struct {
	Name        string
	Pattern     *regexp.Regexp
	Replacement string
	Severity    PIISeverity
}

type MaskingRule struct {
	Action   MaskingAction
	Pattern  string
	Severity PIISeverity
}

type GlobalMaskingRule struct {
	FieldPattern *regexp.Regexp
	ValuePattern *regexp.Regexp
	Action       MaskingAction
	Severity     PIISeverity
}

type PIISeverity int

const (
	PIILow PIISeverity = iota
	PIIMedium
	PIIHigh
	PIICritical
)

type MaskingAction int

const (
	MaskPartial MaskingAction = iota
	MaskFull
	MaskHash
	MaskRemove
	MaskEncrypt
)
