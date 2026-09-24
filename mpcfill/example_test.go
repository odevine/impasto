package mpcfill_test

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/odevine/impasto/canvas"
	"github.com/odevine/impasto/mpcfill"
	"github.com/odevine/impasto/raster"
)

// card builds a solid-colour document at MPC Autofill's 300 DPI convention,
// where a card with bleed is 1110 pixels tall
func card(r, g, b float32) *canvas.Document {
	const w, h = 816, 1110
	buf := raster.MustNewBuffer(w, h)
	for i := 0; i < len(buf.Pix); i += 4 {
		buf.Pix[i], buf.Pix[i+1], buf.Pix[i+2], buf.Pix[i+3] = r, g, b, 1
	}
	return &canvas.Document{
		Width: w, Height: h,
		Root: canvas.Group{PassThrough: true, Opacity: 1, Layers: []canvas.Node{&canvas.Layer{Content: buf}}},
	}
}

// A project with a shared cardback and one double-faced card
func ExampleWrite() {
	dir, _ := os.MkdirTemp("", "mpcfill")
	defer os.RemoveAll(dir)

	transformed := mpcfill.Face{Name: "Insectile Aberration", Document: card(0.1, 0.4, 0.2)}
	err := mpcfill.Write(dir, mpcfill.Project{
		Stock:    mpcfill.S30,
		Cardback: &mpcfill.Face{Name: "House Back", Document: card(0.1, 0.1, 0.3)},
		Cards: []mpcfill.Card{
			{Front: mpcfill.Face{Name: "Island", Document: card(0.1, 0.3, 0.8)}, Quantity: 4},
			{Front: mpcfill.Face{Name: "Delver of Secrets", Document: card(0.2, 0.5, 0.9)}, Back: &transformed},
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	files, _ := filepath.Glob(filepath.Join(dir, "*", "*.png"))
	for _, f := range files {
		rel, _ := filepath.Rel(dir, f)
		fmt.Println(filepath.ToSlash(rel))
	}
	order, _ := os.ReadFile(filepath.Join(dir, mpcfill.OrderFile))
	fmt.Print(string(order))
	// Output:
	// backs/Insectile Aberration.png
	// cardback/House Back.png
	// fronts/Delver of Secrets.png
	// fronts/Island.png
	// <?xml version="1.0" encoding="UTF-8"?>
	// <order>
	//     <details>
	//         <quantity>5</quantity>
	//         <stock>(S30) Standard Smooth</stock>
	//         <foil>false</foil>
	//     </details>
	//     <fronts>
	//         <card>
	//             <id>./fronts/Island.png</id>
	//             <sourceType>Local File</sourceType>
	//             <slots>0,1,2,3</slots>
	//             <name>Island.png</name>
	//             <query>island</query>
	//         </card>
	//         <card>
	//             <id>./fronts/Delver of Secrets.png</id>
	//             <sourceType>Local File</sourceType>
	//             <slots>4</slots>
	//             <name>Delver of Secrets.png</name>
	//             <query>delver of secrets</query>
	//         </card>
	//     </fronts>
	//     <backs>
	//         <card>
	//             <id>./backs/Insectile Aberration.png</id>
	//             <sourceType>Local File</sourceType>
	//             <slots>4</slots>
	//             <name>Insectile Aberration.png</name>
	//             <query>insectile aberration</query>
	//         </card>
	//     </backs>
	//     <cardback>./cardback/House Back.png</cardback>
	// </order>
}

// Write checks the whole project before touching the filesystem, and each
// validation failure wraps one of the package's sentinel errors
func ExampleWrite_validation() {
	err := mpcfill.Write(filepath.Join(os.TempDir(), "mpcfill-never-written"), mpcfill.Project{
		Stock: mpcfill.P10,
		Foil:  true,
		Cards: []mpcfill.Card{{Front: mpcfill.Face{Name: "A", Document: card(1, 1, 1)}}},
	})
	fmt.Println(err)
	// Output:
	// mpcfill: cardstock does not support foil: (P10) Plastic
}
