package env

import (
	"testing"

	"github.com/alecthomas/assert/v2"
)

func TestEnvValidate(t *testing.T) {
	SetEnv(false)
	e := NewE()

	if err := e.Validate("JWT_SECRET"); err != nil {
		t.Fatal(err)
	}
}

func TestHasDotEnvFile(t *testing.T) {
	assert.True(t, true, HasDotEnv())
}

func TestEnvGet(t *testing.T) {
	SetEnv(false)
	e := NewE()

	client, err := e.Get("CLIENT")
	if err != nil {
		t.Fail()
	}
	if client == "" {
		t.Fail()
	}
}

func TestEnvSet(t *testing.T) {
	SetEnv(false)
	e := NewE()

	if err := e.Set("CLIENT", "wienermann nugget"); err != nil {
		t.Fail()
	}

	test, err := e.Get("CLIENT")
	if err != nil {
		t.Fail()
	}
	if test == "" {
		t.Fail()
	}
	if test != "wienermann nugget" {
		t.Fail()
	}
}
