package mpcfill

import (
	"encoding/xml"
	"errors"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/raster"
)

func solid(w, h int, r, g, b float32) *canvas.Document {
	buf := raster.MustNewBuffer(w, h)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = r, g, b, 1
	}
	return &canvas.Document{
		Width: w, Height: h,
		Root: canvas.Group{PassThrough: true, Opacity: 1, Layers: []canvas.Node{&canvas.Layer{Content: buf}}},
	}
}

func face(name string) Face { return Face{Name: name, Document: solid(8, 11, 0.5, 0.2, 0.1)} }

func TestStem(t *testing.T) {
	cases := map[string]string{
		"Lightning Bolt":           "Lightning Bolt",
		"Fire // Ice":              "Fire Ice",
		"Island (Unsanctioned)":    "Island Unsanctioned",
		"{EN} Goblin [NSFW]":       "EN Goblin NSFW",
		"Urza's Saga":              "Urza's Saga",
		"Mishra’s Factory":         "Mishra's Factory",
		"Half-Elf":                 "Half-Elf",
		"Æther Vial":               "AEther Vial",
		"Jötun Grunt":              "Jotun Grunt",
		"日本語":                      "Card",
		"  a.b.c  ":                "a b c",
		"!hidden":                  "hidden",
		"card@id":                  "card id",
		"123":                      "Card",
		"":                         "Card",
		"---":                      "Card",
		"Séance":                   "Seance",
		"b:Cardback":               "b Cardback",
		"Tab\tand\nnewline  runs ": "Tab and newline runs",
	}
	for in, want := range cases {
		if got := stem(in); got != want {
			t.Errorf("stem(%q) = %q want %q", in, got, want)
		}
	}
	if got := stem(strings.Repeat("ab", 200)); len(got) != maxStem {
		t.Errorf("long stem has length %d want %d", len(got), maxStem)
	}
	a := strings.Repeat("a", maxStem-2)
	if got := stem(a + " b"); got != a+" b" {
		t.Errorf("a stem of exactly maxStem was cut: %q", got)
	}
	if got := stem(a + "a b"); got != a+"a" {
		t.Errorf("a separator crossing maxStem kept: %q", got)
	}
}

// FuzzStem checks the properties the website's search relies on: at least one
// letter, and nothing it strips or reinterprets
func FuzzStem(f *testing.F) {
	for _, s := range []string{"Fire // Ice", "Island (Unsanctioned)", "{EN} x", "a.b", "", "Æ"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, name string) {
		s := stem(name)
		if s == "" || len(s) > maxStem {
			t.Fatalf("stem(%q) = %q has bad length", name, s)
		}
		if strings.TrimSpace(s) != s || strings.Contains(s, "  ") {
			t.Fatalf("stem(%q) = %q has stray whitespace", name, s)
		}
		if !strings.ContainsAny(strings.ToLower(s), "abcdefghijklmnopqrstuvwxyz") {
			t.Fatalf("stem(%q) = %q has no letter", name, s)
		}
		for _, r := range s {
			ok := 'a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' ||
				r == ' ' || r == '-' || r == '\''
			if !ok {
				t.Fatalf("stem(%q) = %q contains %q", name, s, r)
			}
		}
	})
}

func TestNamesDeduplicateCaseInsensitively(t *testing.T) {
	n := names{}
	var got []string
	for _, name := range []string{"Goblin", "goblin", "GOBLIN!", "Goblin 2", "Con", "com1", "LPT9", "Console"} {
		got = append(got, n.entry(FrontsDir, face(name), nil).path)
	}
	want := []string{
		"fronts/Goblin.png", "fronts/goblin 2.png", "fronts/GOBLIN 3.png", "fronts/Goblin 2 2.png",
		"fronts/Con 2.png", "fronts/com1 2.png", "fronts/LPT9 2.png", "fronts/Console.png",
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d path = %q want %q", i, got[i], want[i])
		}
	}
}

func TestSlotsAreConsecutive(t *testing.T) {
	back := face("Transformed")
	l, err := Project{
		Cardback: &Face{Name: "Back", Document: solid(8, 11, 0, 0, 0)},
		Cards: []Card{
			{Front: face("A"), Quantity: 2},
			{Front: face("B"), Back: &back},
			{Front: face("C"), Quantity: -4},
			{Front: face("D"), Quantity: 3},
		},
	}.layout()
	if err != nil {
		t.Fatal(err)
	}
	if l.quantity != 7 {
		t.Errorf("quantity = %d want 7", l.quantity)
	}
	want := [][]int{{0, 1}, {2}, {3}, {4, 5, 6}}
	for i, e := range l.fronts {
		if !slices.Equal(e.slots, want[i]) {
			t.Errorf("front %d slots = %v want %v", i, e.slots, want[i])
		}
	}
	if len(l.backs) != 1 || !slices.Equal(l.backs[0].slots, []int{2}) {
		t.Errorf("backs = %+v want one back in slot 2", l.backs)
	}
	if l.stock != S30 {
		t.Errorf("zero stock resolved to %q want %q", l.stock, S30)
	}
}

func TestValidation(t *testing.T) {
	cb := face("Back")
	nilDoc := Face{Name: "Empty"}
	cases := []struct {
		name string
		p    Project
		want error
	}{
		{"no cards", Project{Cardback: &cb}, ErrNoCards},
		{"unknown stock", Project{Stock: "S30", Cardback: &cb, Cards: []Card{{Front: face("A")}}}, ErrStock},
		{"foil plastic", Project{Stock: P10, Foil: true, Cardback: &cb, Cards: []Card{{Front: face("A")}}}, ErrFoil},
		{"no cardback", Project{Cards: []Card{{Front: face("A")}}}, ErrNoCardback},
		{"nil front", Project{Cardback: &cb, Cards: []Card{{Front: nilDoc}}}, ErrNoDocument},
		{"nil back", Project{Cardback: &cb, Cards: []Card{{Front: face("A"), Back: &nilDoc}}}, ErrNoDocument},
		{"nil cardback", Project{Cardback: &nilDoc, Cards: []Card{{Front: face("A")}}}, ErrNoDocument},
		{"empty front", Project{Cardback: &cb, Cards: []Card{{Front: Face{Document: &canvas.Document{Height: 10}}}}}, ErrDocumentSize},
		{"huge back", Project{Cardback: &cb, Cards: []Card{{Front: face("A"), Back: &Face{Document: &canvas.Document{
			Width: 10, Height: raster.MaxDimension + 1,
		}}}}}, ErrDocumentSize},
		{"over max", Project{Cardback: &cb, Cards: []Card{{Front: face("A"), Quantity: MaxProjectSize + 1}}}, ErrTooManyCards},
		{"huge quantity", Project{Cardback: &cb, Cards: []Card{{Front: face("A"), Quantity: 1 << 40}}}, ErrTooManyCards},
		{"over max across cards", Project{Cardback: &cb, Cards: []Card{
			{Front: face("A"), Quantity: MaxProjectSize}, {Front: face("B")},
		}}, ErrTooManyCards},
	}
	for _, c := range cases {
		dir := filepath.Join(t.TempDir(), "project")
		err := Write(dir, c.p)
		if !errors.Is(err, c.want) {
			t.Errorf("%s: err = %v want %v", c.name, err, c.want)
		}
		if _, statErr := os.Stat(dir); !os.IsNotExist(statErr) {
			t.Errorf("%s: invalid project touched the filesystem", c.name)
		}
	}
}

func TestProjectLimits(t *testing.T) {
	cb := face("Back")
	if _, err := (Project{Cardback: &cb, Cards: []Card{{Front: face("A"), Quantity: MaxProjectSize}}}).layout(); err != nil {
		t.Errorf("a project of exactly MaxProjectSize should be valid: %v", err)
	}
	for _, s := range []Stock{S27, S30, S33, M31} {
		if _, err := (Project{Stock: s, Foil: true, Cardback: &cb, Cards: []Card{{Front: face("A")}}}).layout(); err != nil {
			t.Errorf("foil %s should be valid: %v", s, err)
		}
	}
	back := face("B back")
	if _, err := (Project{Cards: []Card{{Front: face("B"), Back: &back}}}).layout(); err != nil {
		t.Errorf("an all double-faced project needs no cardback: %v", err)
	}
	if _, err := (Project{Cardback: &cb, Cards: []Card{{Front: face("B"), Back: &back}}}).layout(); err != nil {
		t.Errorf("an all double-faced project may still have a cardback: %v", err)
	}
}

func TestFrontAndBackNamesAreIndependent(t *testing.T) {
	back := face("Delver")
	l, err := Project{Cards: []Card{{Front: face("Delver"), Back: &back}}}.layout()
	if err != nil {
		t.Fatal(err)
	}
	if l.fronts[0].path != "fronts/Delver.png" || l.backs[0].path != "backs/Delver.png" {
		t.Errorf("paths = %q, %q", l.fronts[0].path, l.backs[0].path)
	}
}

func TestWrite(t *testing.T) {
	dir := t.TempDir()
	back := Face{Name: "Delver // Aberration", Document: solid(8, 11, 0, 0.5, 0)}
	p := Project{
		Stock:    S33,
		Foil:     true,
		Cardback: &Face{Name: "Classic Back", Document: solid(8, 11, 0.2, 0.1, 0)},
		Cards: []Card{
			{Front: face("Island (Unsanctioned)"), Quantity: 3},
			{Front: face("Delver of Secrets"), Back: &back},
			{Front: face("island unsanctioned")},
		},
	}
	if err := Write(dir, p); err != nil {
		t.Fatal(err)
	}

	got, err := os.ReadFile(filepath.Join(dir, OrderFile))
	if err != nil {
		t.Fatal(err)
	}
	want := `<?xml version="1.0" encoding="UTF-8"?>
<order>
    <details>
        <quantity>5</quantity>
        <stock>(S33) Superior Smooth</stock>
        <foil>true</foil>
    </details>
    <fronts>
        <card>
            <id>./fronts/Island Unsanctioned.png</id>
            <sourceType>Local File</sourceType>
            <slots>0,1,2</slots>
            <name>Island Unsanctioned.png</name>
            <query>island unsanctioned</query>
        </card>
        <card>
            <id>./fronts/Delver of Secrets.png</id>
            <sourceType>Local File</sourceType>
            <slots>3</slots>
            <name>Delver of Secrets.png</name>
            <query>delver of secrets</query>
        </card>
        <card>
            <id>./fronts/island unsanctioned 2.png</id>
            <sourceType>Local File</sourceType>
            <slots>4</slots>
            <name>island unsanctioned 2.png</name>
            <query>island unsanctioned 2</query>
        </card>
    </fronts>
    <backs>
        <card>
            <id>./backs/Delver Aberration.png</id>
            <sourceType>Local File</sourceType>
            <slots>3</slots>
            <name>Delver Aberration.png</name>
            <query>delver aberration</query>
        </card>
    </backs>
    <cardback>./cardback/Classic Back.png</cardback>
</order>
`
	if string(got) != want {
		t.Errorf("order file:\n%s\nwant:\n%s", got, want)
	}

	// Every id must name a PNG relative to dir, as the desktop tool reads it
	// after changing into dir
	var o xmlOrder
	if err := xml.Unmarshal(got, &o); err != nil {
		t.Fatal(err)
	}
	ids := []string{o.Cardback}
	for _, c := range append(o.Fronts.Cards, o.Backs.Cards...) {
		ids = append(ids, c.ID)
	}
	for _, id := range ids {
		f, err := os.Open(filepath.Join(dir, filepath.FromSlash(id)))
		if err != nil {
			t.Errorf("%s: %v", id, err)
			continue
		}
		cfg, err := png.DecodeConfig(f)
		f.Close()
		if err != nil || cfg.Width != 8 || cfg.Height != 11 {
			t.Errorf("%s: %dx%d, %v; want an 8x11 PNG", id, cfg.Width, cfg.Height, err)
		}
	}
}

func TestWriteOmitsUnusedElements(t *testing.T) {
	dir := t.TempDir()
	back := face("B back")
	if err := Write(dir, Project{Cards: []Card{{Front: face("B"), Back: &back}}}); err != nil {
		t.Fatal(err)
	}
	out, _ := os.ReadFile(filepath.Join(dir, OrderFile))
	if strings.Contains(string(out), "<cardback") {
		t.Errorf("order without a cardback has a cardback element:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dir, CardbackDir)); !os.IsNotExist(err) {
		t.Errorf("cardback folder created for a project without a cardback")
	}

	dir = t.TempDir()
	cb := face("Back")
	if err := Write(dir, Project{Cardback: &cb, Cards: []Card{{Front: face("A")}}}); err != nil {
		t.Fatal(err)
	}
	out, _ = os.ReadFile(filepath.Join(dir, OrderFile))
	if strings.Contains(string(out), "<backs") {
		t.Errorf("order without double-faced cards has a backs element:\n%s", out)
	}
}
