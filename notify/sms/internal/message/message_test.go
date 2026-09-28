package message

import (
	"testing"

	notify "github.com/liujitcn/kratos-kit/notify"
)

func TestOrderedParamsRequiresStableOrder(t *testing.T) {
	_, err := OrderedParams(&notify.Template{Code: "tpl", Params: map[string]string{"code": "1234"}})
	if err == nil {
		t.Fatal("expected ordered parameters to be required")
	}
	got, err := OrderedParams(&notify.Template{Code: "tpl", OrderedParams: []string{"first", "second"}})
	if err != nil || len(got) != 2 || got[0] != "first" || got[1] != "second" {
		t.Fatalf("unexpected ordered parameters: %v, %v", got, err)
	}
}
