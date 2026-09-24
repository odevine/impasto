# mpcfill

`mpcfill` writes rendered cards out as an [MPC Autofill](https://github.com/chilli-axe/mpc-autofill)
project: a directory of PNGs plus the `cards.xml` order file MPC Autofill uses to
place an order with MakePlayingCards. It sits above `canvas`, nothing else
imports it, and it is the one place impasto holds an opinion about a file format.

```go
import "github.com/odevine/impasto/mpcfill"
```

## The model

```go
type Face struct {
    Name     string
    Document *canvas.Document
}

type Card struct {
    Front    Face
    Back     *Face  // nil: the project cardback. Set: a double-faced card
    Quantity int    // copies; non-positive counts as one
}

type Project struct {
    Stock    Stock  // zero value is S30
    Foil     bool
    Cardback *Face  // shared by every card without its own Back
    Cards    []Card
}

err := mpcfill.Write(dir, project)
```

A face is a `canvas.Document`, not a rendered buffer. `Write` renders each face
once, encodes it, and drops the pixels before moving to the next, so a project
never holds more than one rendered card in memory, however many cards it has.
A face used for 60 copies of a card is still rendered once: set `Quantity`
rather than repeating the card.

Every card shares the project's `Cardback` unless it has a `Back` of its own,
which is how double-faced cards are expressed. `Cardback` may be nil only when
every card is double-faced.

## What gets written

```
dir/cards.xml
dir/fronts/<name>.png     card fronts
dir/backs/<name>.png      backs of double-faced cards
dir/cardback/<name>.png   the shared cardback
```

Cards take consecutive slots in slice order, so a project of `{A ×3, B, C ×2}`
puts A in slots 0 to 2, B in 3, and C in 4 and 5. The order file refers to every
image by its path relative to `dir`, for example `./fronts/Island.png`.

The project is validated before anything touches the filesystem, so an invalid
project leaves no partial output. Existing files are overwritten but never
removed. Write into an empty directory, because the website indexes every image
it finds there.

## Using the output

**Desktop tool.** Point it at the directory with its `-d dir` flag, or run it
from inside `dir`. It changes into that directory, finds `cards.xml`, and
resolves each image path relative to it.

**Website.** Open *Sources*, choose `dir` under *Local Folder*, then import
`cards.xml` with the XML importer, under *Add Cards → XML*, or the *XML* panel
an empty project shows. The website only knows the images in a
connected folder, so the folder has to be connected before the import. The
website also refuses an import that would take the project past 612 cards, so
import into an empty project.

## Why names are rewritten

`Face.Name` becomes the file name, but not verbatim. It keeps ASCII letters,
digits, hyphens, and apostrophes. Accented Latin letters are written in plain
ASCII first. Everything else, including any other non-ASCII letter, becomes a
single space:

| Name                         | File                             |
| ---------------------------- | -------------------------------- |
| `Lightning Bolt`             | `Lightning Bolt.png`             |
| `Fire // Ice`                | `Fire Ice.png`                   |
| `Island (Unsanctioned)`      | `Island Unsanctioned.png`        |
| `Æther Vial`                 | `AEther Vial.png`                |
| `Séance`                     | `Seance.png`                     |
| `Mishra’s Factory`           | `Mishra's Factory.png`           |
| `123`, `!!!`, `日本語`       | `Card.png`                       |

Names that collide once rewritten, compared case-insensitively, get a numeric
suffix: `Goblin.png`, `goblin 2.png`, `GOBLIN 3.png`. So do Windows device
names such as `Con` and `COM1`, which Windows refuses as file names. Names are
capped at 100 characters.

The reason is how the website imports an order. For each card it reads the
image id, but also the card's search query, and once the import finishes it
runs that query. **If the imported image is not among the query's results, the
website swaps in the first result** and lists the card under *Invalid
Identifiers*. If the query finds nothing at all, the image is dropped without
even that. So a card only survives the import if searching for its own query
finds its own file.

The website's search normalizes both sides: it lowercases, drops digits and
punctuation, deletes anything in brackets, and reads `(...)` in a query as a
set code. A name containing any of that can produce a query that
no longer finds its file. `mpcfill` sidesteps all of it by writing only
characters that survive normalization unchanged, then using the lowercased file
name as the query. The two always match.

ASCII in particular is about identity, not search. The order file has to name
each image byte for byte, and a non-ASCII file name can come back from the
filesystem in a different Unicode normalization than the one written.

## Why the fixed folders

The website decides whether an image is a card or a cardback from the name of
the folder that directly holds it: a folder whose name contains "cardback" holds
cardbacks, one containing "token" holds tokens. Two things follow.

The cardback has to live in a folder named for it, or the website will not
offer it as a cardback. And fronts cannot sit in the project root, because the
root's name is whatever the caller chose. Writing fronts to the root of a
directory called `goblin tokens` would make the website retype every card.

The folder names avoid everything else the website strips or reads from folder
names: `{EN} ` language prefixes, `[tag]` and `(tag)` groups, and a leading `!`,
which hides a folder from the index entirely.

## Limits

| Rule                                     | Error             |
| ---------------------------------------- | ----------------- |
| At least one card                        | `ErrNoCards`      |
| At most `MaxProjectSize` (612) cards     | `ErrTooManyCards` |
| `Stock` is one of the five constants     | `ErrStock`        |
| No foil on `P10`                         | `ErrFoil`         |
| A cardback unless every card has a back  | `ErrNoCardback`   |
| Every face has a `Document`              | `ErrNoDocument`   |
| Every `Document` has a renderable size   | `ErrDocumentSize` |

612 is MPC Autofill's own ceiling. The website cannot import a larger order, and
the desktop tool only splits orders when it combines several order files. For a
bigger order, split the cards across several projects, each in its own
directory.

`Stock` values are MakePlayingCards' exact names, such as `(S30) Standard Smooth`,
because both tools compare the string exactly.

## Image size

`Write` renders a document at whatever size it has. MPC Autofill measures
resolution by height, treating a 1110-pixel-tall card as 300 DPI. A
full-bleed card at 300 DPI is about 816×1111 pixels: the 63×88 mm card plus
0.12 inches (3.048 mm) of bleed on each side. The website hides images above 1500 DPI (about
5550 pixels tall) or 30 MB with its default search settings, and an image it
hides cannot survive an import.

## What is not here

- **Images hosted elsewhere.** Every image is a local file written by `Write`.
  Google Drive ids, which the format also allows, are not supported.
- **Tokens.** Tokens print like any other card, so they go in `Cards`. The
  website will list them as cards, not tokens.
- **Splitting large orders.** Past 612 cards, `Write` returns an error rather
  than choosing split points, since where to split affects MakePlayingCards'
  pricing brackets.
