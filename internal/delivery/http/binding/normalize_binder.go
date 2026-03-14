// Package binding provides Echo binding helpers. The NormalizeBinder wraps any
// Binder and normalizes string fields after binding: trim leading/trailing
// whitespace, and optional case via tags. Use trim:"false" to opt out of trim.
// Use case:"lower" or case:"upper" to normalize case after trim; omit to leave
// case unchanged.
package binding

import (
	"reflect"
	"strings"

	"github.com/labstack/echo/v5"
)

const (
	trimOptOutTag = "trim" // value "false" = do not trim
	caseTag       = "case" // value "lower" or "upper" = normalize case after trim
)

type normalizeBinder struct {
	inner echo.Binder
}

// NewNormalizeBinder returns a Binder that delegates to inner then normalizes
// string fields (trim and optional case). If inner is nil, echo.DefaultBinder is used.
func NewNormalizeBinder(inner echo.Binder) echo.Binder {
	if inner == nil {
		inner = &echo.DefaultBinder{}
	}
	return &normalizeBinder{inner: inner}
}

// Bind implements echo.Binder.
func (b *normalizeBinder) Bind(c *echo.Context, target any) error {
	if err := b.inner.Bind(c, target); err != nil {
		return err
	}
	normalizeStrings(target)
	return nil
}

// normalizeStrings walks target (expected pointer to struct), trims string fields
// (unless trim:"false"), and applies case normalization when case:"lower" or
// case:"upper" is set. Recurses into nested structs, pointer-to-struct, and
// slice elements ([]string, []struct, []*struct).
func normalizeStrings(target any) {
	if target == nil {
		return
	}
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr {
		return
	}
	v = v.Elem()
	if v.Kind() != reflect.Struct {
		return
	}
	normalizeStruct(v)
}

func normalizeStruct(v reflect.Value) {
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if !field.CanSet() {
			continue
		}
		switch field.Kind() {
		case reflect.String:
			sf := t.Field(i)
			s := field.String()
			if !skipTrim(sf) {
				s = strings.TrimSpace(s)
			}
			field.SetString(applyCase(sf, s))
		case reflect.Struct:
			normalizeStruct(field)
		case reflect.Ptr:
			if field.Type().Elem().Kind() == reflect.Struct {
				normalizeStruct(field)
			}
		case reflect.Slice:
			normalizeSlice(field)
		}
	}
}

func normalizeSlice(v reflect.Value) {
	if v.Kind() != reflect.Slice || v.IsNil() {
		return
	}
	elemKind := v.Type().Elem().Kind()
	for i := 0; i < v.Len(); i++ {
		el := v.Index(i)
		switch elemKind {
		case reflect.String:
			if el.CanSet() {
				el.SetString(strings.TrimSpace(el.String()))
			}
		case reflect.Struct:
			normalizeStruct(el)
		case reflect.Ptr:
			if el.Type().Elem().Kind() == reflect.Struct {
				normalizeStruct(el)
			}
		}
	}
}

func skipTrim(sf reflect.StructField) bool {
	return sf.Tag.Get(trimOptOutTag) == "false"
}

// applyCase applies case normalization from the case tag; "lower" or "upper".
// Unknown or empty tag returns s unchanged.
func applyCase(sf reflect.StructField, s string) string {
	switch sf.Tag.Get(caseTag) {
	case "lower":
		return strings.ToLower(s)
	case "upper":
		return strings.ToUpper(s)
	default:
		return s
	}
}
