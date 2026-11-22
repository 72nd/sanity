//go:build !darwin

package pdf

import "image"

type stub struct{}

// NewEngine returns an Engine whose methods all fail with ErrUnsupported.
func NewEngine() Engine { return stub{} }

func (stub) Meta(string) (Meta, error)                        { return Meta{}, ErrUnsupported }
func (stub) Render(string, int, float64) (image.Image, error) { return nil, ErrUnsupported }
func (stub) Text(string, int) (PageText, error)               { return PageText{}, ErrUnsupported }
func (stub) Dates(string) []string                            { return nil }
func (stub) OCRAvailable() bool                               { return false }

// OCRPage is only implemented on macOS.
func OCRPage(string, int, OCROptions) (string, error) { return "", ErrUnsupported }
