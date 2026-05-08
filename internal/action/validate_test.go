package action

import (
	"strings"
	"testing"
)

func TestValidateReleaseName_Valid(t *testing.T) {
	validNames := []string{
		"a",
		"z",
		"0",
		"my-app",
		"my-app-123",
		"a1",
		"abc",
		"release-name-with-dashes",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateReleaseName(name)
			if nil != err {
				t.Errorf("expected name %q to be valid, got error: %v", name, err)
			}
		})
	}
}

func TestValidateReleaseName_Empty(t *testing.T) {
	err := ValidateReleaseName("")
	if nil == err {
		t.Fatal("expected error for empty name")
	}
	if !strings.Contains(err.Error(), "required") {
		t.Errorf("expected error to mention 'required', got: %v", err)
	}
}

func TestValidateReleaseName_TooLong(t *testing.T) {
	longName := strings.Repeat("a", 54)
	err := ValidateReleaseName(longName)
	if nil == err {
		t.Fatal("expected error for name exceeding 53 characters")
	}
	if !strings.Contains(err.Error(), "exceeds maximum length") {
		t.Errorf("expected error about max length, got: %v", err)
	}
}

func TestValidateReleaseName_ExactlyMaxLength(t *testing.T) {
	name := strings.Repeat("a", 53)
	err := ValidateReleaseName(name)
	if nil != err {
		t.Errorf("expected 53-char name to be valid, got: %v", err)
	}
}

func TestValidateReleaseName_InvalidCharacters(t *testing.T) {
	invalidNames := []string{
		"My-App",
		"my_app",
		"-my-app",
		"my-app-",
		"my.app",
		"my app",
		"MY-APP",
	}

	for _, name := range invalidNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateReleaseName(name)
			if nil == err {
				t.Errorf("expected name %q to be invalid", name)
			}
		})
	}
}

func TestValidateReleaseName_SingleChar(t *testing.T) {
	singleChars := []string{"a", "z", "0", "9"}
	for _, ch := range singleChars {
		t.Run(ch, func(t *testing.T) {
			err := ValidateReleaseName(ch)
			if nil != err {
				t.Errorf("expected single char %q to be valid, got: %v", ch, err)
			}
		})
	}
}
