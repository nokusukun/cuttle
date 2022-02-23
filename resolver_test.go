package cuttle

import (
	"fmt"
	"testing"
)

func TestGetNameTag(t *testing.T) {

	type A struct {
		Foo string `bind:"query,json,header"`
	}

	got, got1 := GetNameTag(&A{Foo: "afoo"}, 0)
	fmt.Println(got, got1)
}
