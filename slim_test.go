package slim

import (
	"bytes"
	"testing"
)

func TestSimple(t *testing.T) {
	tmpl, err := ParseFile("testdir/test1.slim")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		t.Fatal(err)
	}
	expect := `<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8"/>
    <title>
    </title>
  </head>
  <body>
    <p>Hello</p>
  </body>
</html>
`
	got := buf.String()
	if expect != got {
		t.Fatalf("expected %v but %v", expect, got)
	}
}

func TestValue(t *testing.T) {
	tmpl, err := ParseFile("testdir/test2.slim")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"foo": "bar",
	})
	if err != nil {
		t.Fatal(err)
	}
	expect := `<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8"/>
    <title>
    </title>
  </head>
  <body>
    <p>bar</p>
  </body>
</html>
`
	got := buf.String()
	if expect != got {
		t.Fatalf("expected %v but %v", expect, got)
	}
}

func TestUnknownIdentifier(t *testing.T) {
	tmpl, err := ParseFile("testdir/test2.slim")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"bar": "baz",
	})
	if err == nil {
		t.Fatal("should be fail")
	}
}

func TestEach(t *testing.T) {
	tmpl, err := ParseFile("testdir/test_each.slim")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]interface{}{
		"foo": []string{"foo", "bar", "baz"},
	})
	if err != nil {
		t.Fatal(err)
	}
	expect := `<!doctype html>
<html lang="ja">
  <head>
    <meta charset="UTF-8"/>
    <title>
    </title>
  </head>
  <body>
    <ul>
      <li>foo</li>
      <li>bar</li>
      <li>baz</li>
    </ul>
  </body>
</html>
`
	got := buf.String()
	if expect != got {
		t.Fatalf("expected %v but %v", expect, got)
	}
}
