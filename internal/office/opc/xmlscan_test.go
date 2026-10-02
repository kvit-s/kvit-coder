package opc

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var scanEdgeCases = []string{
	`<a/>`,
	`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" + `<r xmlns="urn:x"><c a='1' b = "2"/>text</r>`,
	`<?xml version="1.1"?><r/>`,
	`<?xml version="1.0" encoding="ISO-8859-1"?><r>caf` + "\xe9" + `</r>`,
	`<?xml version="1.0" encoding="utf-16"?><r/>`,
	`<?xml-stylesheet href="s.xsl"?><r/>`,
	`<? xml?><r/>`,
	`<!DOCTYPE r [<!ENTITY e "x">]><r>&e;</r>`,
	`<r><![CDATA[a<b]]></r>`,
	`<r>a]]>b</r>`,
	`<r a="x]]>y"/>`,
	`<r>&lt;&gt;&amp;&apos;&quot;&#65;&#x42;&#X43;&#x;&#;&#0;&#xD800;&#x10FFFF;&#1114112;</r>`,
	`<r>&nbsp;&copy;&unknown;&amp x;&#65x;&</r>`,
	`<r a="&#9;&#10;&#13;" b="line` + "\r\n" + `next` + "\r" + `last"/>`,
	"<r>one\r\ntwo\r\rthree\r&#10;four</r>",
	`<r>a<!-- c --> b<!----> c<?pi data?>d</r>`,
	`<r><!-- a--b --></r>`,
	`<r><!---></r>`,
	`<r><!-x></r>`,
	`<r a=unquoted b/>`,
	`<r a="<"/>`,
	`<r a="1"b="2"/>`,
	`<r/ >`,
	`<1r/>`,
	`<:r/>`,
	`<a:b:c/>`,
	`<a: x:="1" _y="2" z.w-v="3"/>`,
	`<r><é/></r>`,
	`<ré/>`,
	`< r/>`,
	`<r></r >`,
	`<r></r x>`,
	`<r><a><b></a></r>`,
	`<r><a>unclosed`,
	`</stray><r/>`,
	`<r/><second><kid/></second>`,
	`text only`,
	``,
	`<r>` + "\x00" + `</r>`,
	`<r>` + "\xff" + `</r>`,
	`<r>` + "\uffff" + `</r>`,
	"\t<r>\u00a0\ufeff\U0001F600</r>\n",
	`<r xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:a14="urn:a14"><mc:Choice Requires="a14 zz"/></r>`,
	`<x:r xmlns:x="http://purl.oclc.org/ooxml/wordprocessingml/main" xml:space="preserve" x:a="1"/>`,
	`<r><`,
	`<r a="1`,
	`<r a`,
	`<r a=`,
	`<r><?pi`,
	`<r><!--`,
}

func sameParse(t *testing.T, name string, data []byte, maxNodes int) bool {
	t.Helper()
	root, err := scanXML(data, maxNodes)
	if errors.Is(err, errNotScannable) {
		return false
	}
	wantRoot, wantErr := decodeXML(data, maxNodes)
	if !reflect.DeepEqual(err, wantErr) || !reflect.DeepEqual(root, wantRoot) {
		t.Errorf("%s: scanner and decoder disagree\nscan:   %v %s\ndecode: %v %s", name, err, dump(root), wantErr, dump(wantRoot))
	}
	return true
}

func dump(e *Element) string {
	if e == nil {
		return "<nil>"
	}
	var sb strings.Builder
	var walk func(*Element)
	walk = func(e *Element) {
		sb.WriteString("{" + e.Space + " " + e.Local)
		for _, a := range e.Attrs {
			sb.WriteString(" " + a.Space + ":" + a.Local + "=" + a.Value)
		}
		for _, n := range e.Nodes {
			if n.Elem != nil {
				walk(n.Elem)
			} else {
				sb.WriteString(" " + n.Text)
			}
		}
		sb.WriteString("}")
	}
	walk(e)
	return sb.String()
}

func TestScannerMatchesDecoderOnEdgeCases(t *testing.T) {
	for _, c := range scanEdgeCases {
		sameParse(t, c, []byte(c), MaxXMLNodes)
		for _, cap := range []int{0, 1, 2, 3} {
			sameParse(t, c, []byte(c), cap)
		}
	}
	deep := strings.Repeat("<a>", MaxXMLDepth+1)
	if !sameParse(t, "deep", []byte(deep), MaxXMLNodes) {
		t.Error("plain deep nesting must take the scanner")
	}
}

func TestScannerMatchesDecoderOnFixtureParts(t *testing.T) {
	// The fixture corpus lives at the root of the upstream repository and was
	// not copied into kvit-coder, so the test runs only when it is present.
	fixtures := filepath.Join("..", "..", "testdata")
	if _, err := os.Stat(fixtures); err != nil {
		t.Skip("upstream fixture corpus not present")
	}
	scanned, total := 0, 0
	err := filepath.WalkDir(fixtures, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".docx", ".xlsx", ".pptx":
		default:
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return nil
		}
		for _, f := range zr.File {
			if !strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".rels") {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				continue
			}
			part, err := io.ReadAll(io.LimitReader(rc, MaxEntryBytes))
			_ = rc.Close()
			if err != nil {
				continue
			}
			total++
			if sameParse(t, path+"!"+f.Name, toUTF8(part), MaxXMLNodes) {
				scanned++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 || scanned*10 < total*9 {
		t.Errorf("scanner took %d of %d fixture parts; most real parts must take it", scanned, total)
	}
}

func FuzzScannerMatchesDecoder(f *testing.F) {
	for _, c := range scanEdgeCases {
		f.Add([]byte(c))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		sameParse(t, "fuzz", data, 64)
	})
}
