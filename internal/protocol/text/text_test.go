package text

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Relixik/gomc/internal/protocol/nbt"
)

func TestPlainNBT(t *testing.T) {
	if got := Plain("hi").NBT(); got != nbt.String("hi") {
		t.Errorf("Plain NBT = %v, want String(\"hi\")", got)
	}
}

func TestStyledNBT(t *testing.T) {
	c := Component{Text: "hi", Color: "red", Bold: Boolp(true)}
	comp, ok := c.NBT().(*nbt.Compound)
	if !ok {
		t.Fatalf("styled NBT is %T, want *nbt.Compound", c.NBT())
	}
	if v, _ := comp.Get("text"); v != nbt.String("hi") {
		t.Errorf("text = %v", v)
	}
	if v, _ := comp.Get("color"); v != nbt.String("red") {
		t.Errorf("color = %v", v)
	}
	if v, _ := comp.Get("bold"); v != nbt.Byte(1) {
		t.Errorf("bold = %v, want Byte(1)", v)
	}
}

func TestNestedExtraNBT(t *testing.T) {
	c := Component{
		Text:  "a",
		Color: "white",
		Extra: []Component{{Text: "b", Color: "blue"}},
	}
	comp := c.NBT().(*nbt.Compound)
	extra, ok := comp.Get("extra")
	if !ok {
		t.Fatal("missing extra")
	}
	list := extra.(nbt.List)
	if list.ElemType != nbt.TagCompound || len(list.Elems) != 1 {
		t.Fatalf("extra list = %+v", list)
	}
	child := list.Elems[0].(*nbt.Compound)
	if v, _ := child.Get("color"); v != nbt.String("blue") {
		t.Errorf("child color = %v", v)
	}
}

func TestMarshalJSONPlain(t *testing.T) {
	b, err := json.Marshal(Plain("hi"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `"hi"` {
		t.Errorf("plain JSON = %s, want \"hi\"", b)
	}
}

func TestMarshalJSONStyled(t *testing.T) {
	b, err := json.Marshal(Component{Text: "hi", Color: "red", Bold: Boolp(true)})
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"text":"hi"`, `"color":"red"`, `"bold":true`} {
		if !strings.Contains(s, want) {
			t.Errorf("styled JSON %s missing %s", s, want)
		}
	}
}

func TestStatusResponseJSON(t *testing.T) {
	resp := StatusResponse{
		Version:     StatusVersion{Name: "26.1.2", Protocol: 775},
		Players:     StatusPlayers{Max: 20, Online: 0},
		Description: Plain("gomc"),
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for _, want := range []string{`"protocol":775`, `"name":"26.1.2"`, `"max":20`, `"description":"gomc"`, `"enforcesSecureChat":false`} {
		if !strings.Contains(s, want) {
			t.Errorf("status JSON %s missing %s", s, want)
		}
	}
}
