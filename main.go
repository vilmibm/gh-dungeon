package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"

	"github.com/AlecAivazis/survey/v2"
	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
)

/*

For friday:

- actually grab room info at the top of the loop for RenderRoom to work with
- PoC flavor system
- Parsing for targeted commands (going through a door, examining a file)

*/

type IOStreams struct {
	In  io.Reader
	Out io.Writer
}

type Opts struct {
	Args   []string
	Repo   string
	IO     *IOStreams
	Getter ContentGetter
	REPL   REPL
}

type Contents struct {
	Files []FileResponse
	Dirs  []string
}

type ContentGetter interface {
	GetDirContents(path string) (Contents, error)
	GetFileContents(path string) (FileResponse, error)
}

type RESTContentGetter struct {
	client api.RESTClient
	repo   string
}

func NewRESTContentGetter(client api.RESTClient, repo string) *RESTContentGetter {
	return &RESTContentGetter{
		client: client,
		repo:   repo,
	}
}

type FileResponse struct {
	Type string
	Name string
}

func (g *RESTContentGetter) GetFileContents(path string) (resp FileResponse, err error) {
	err = g.client.Get(fmt.Sprintf("repos/%s/contents/%s", g.repo, path), &resp)
	return
}

func (g *RESTContentGetter) GetDirContents(path string) (c Contents, err error) {
	c = Contents{
		Files: []FileResponse{},
		Dirs:  []string{},
	}
	var resp []FileResponse
	if err = g.client.Get(fmt.Sprintf("repos/%s/contents/%s", g.repo, path), &resp); err != nil {
		return
	}

	for _, entry := range resp {
		if entry.Type == "file" {
			c.Files = append(c.Files, entry)
		} else if entry.Type == "dir" {
			c.Dirs = append(c.Dirs, entry.Name)
		}
	}

	return
}

func _main(opts *Opts) error {
	g := opts.Getter
	repl := opts.REPL

	state := GameState{
		Repo: opts.Repo,
	}

	var cmd Command
	var err error
	contents, err := g.GetDirContents(state.Path)
	if err != nil {
		panic("TODO handle this")
	}
	state.Contents = contents

	fmt.Fprintln(opts.IO.Out, state.RenderRoom())

	for {
		cmd, err = repl.NextCommand()
		if err != nil && errors.Is(err, UnknownCommandError{}) {
			fmt.Fprintln(opts.IO.Out, err.Error())
			err = nil
			continue
		} else if err != nil {
			break
		}

		if cmd.Kind == LookCommand {
			fmt.Fprintln(opts.IO.Out, state.RenderRoom())
		}

		if cmd.Kind == ExamineCommand {
			if len(state.Contents.Files) == 0 {
				fmt.Fprintln(opts.IO.Out, "you don't see anything to examine in here")
			} else {
				var which FileResponse
				which, err = repl.ExamineWhich(state.Contents.Files)
				if err != nil {
					break
				}
				fmt.Fprintf(opts.IO.Out, "you are holding a paper titled %s.", which.Name)

				// TODO do a confirm then open a pager
			}
		}

		if cmd.Kind == QuitCommand {
			fmt.Fprintln(opts.IO.Out, "see you again~")
			break
		}

		if cmd.Kind == HelpCommand {
			fmt.Fprintln(opts.IO.Out, "supported verbs: look, go, examine, quit")
		}
	}

	// TODO fill in Contents struct as needed
	// TODO hash functions for repo names and file names

	return err
}

type CommandKind string

type Command struct {
	Kind CommandKind
	Args []string
	Raw  string
}

type REPL interface {
	NextCommand() (Command, error)
	ExamineWhich(files []FileResponse) (FileResponse, error)
}

type IOREPL struct {
	io      *IOStreams
	history []Command
}

func NewIOREPL(io *IOStreams) *IOREPL {
	return &IOREPL{
		io:      io,
		history: []Command{},
	}
}

const (
	LookCommand    CommandKind = "look"
	GoCommand      CommandKind = "go"
	QuitCommand    CommandKind = "quit"
	HelpCommand    CommandKind = "help"
	ExamineCommand CommandKind = "examine"
)

func (r *IOREPL) ExamineWhich(files []FileResponse) (FileResponse, error) {
	opts := []string{}
	for _, f := range files {
		opts = append(opts, f.Name)
	}

	fmt.Fprintln(r.io.Out, "you gather up the papers and look at their titles.")

	var selected int
	if err := survey.AskOne(&survey.Select{
		Message: "examine which paper?",
		Options: opts,
	}, &selected); err != nil {
		return FileResponse{}, err
	}

	return files[selected], nil

}

func (r *IOREPL) NextCommand() (cmd Command, err error) {
	raw := ""
	if err = survey.AskOne(&survey.Input{
		Message: "for help >",
	}, &raw); err != nil {
		return
	}

	return parseCommand(raw)
}

type UnknownCommandError struct {
	error
	Raw string
}

func parseCommand(raw string) (cmd Command, err error) {
	// TODO further do parsing and produce Args
	if strings.HasPrefix(raw, "look") {
		cmd = Command{
			Raw:  raw,
			Kind: LookCommand,
		}
	} else if strings.HasPrefix(raw, "examine") {
		cmd = Command{
			Raw:  raw,
			Kind: ExamineCommand,
		}
	} else if strings.HasPrefix(raw, "quit") {
		cmd = Command{
			Raw:  raw,
			Kind: QuitCommand,
		}
	} else if strings.HasPrefix(raw, "go") {
		cmd = Command{
			Raw:  raw,
			Kind: GoCommand,
		}
	} else if strings.HasPrefix(raw, "?") {
		cmd = Command{
			Raw:  raw,
			Kind: HelpCommand,
		}

	} else {
		err = UnknownCommandError{
			Raw: raw,
		}
	}

	return
}

type GameState struct {
	Repo     string
	Path     string
	Contents Contents
}

const roomTmpl string = `
You are standing in a room. {{ .RoomDescription }}

{{ .RoomFlavor }}

{{ .ItemsDesc }}
`

// TODO hallway rendering

func (s *GameState) RenderRoom() string {
	// TODO pay attention to Path
	// TODO render door
	// TODO special handling for root room?
	// TODO objects on ground rendering
	tmpl, err := template.New("room").Parse(roomTmpl)
	if err != nil {
		panic(err.Error())
	}
	out := &bytes.Buffer{}
	desc := "A sign reads %s"
	if s.Path == "" {
		desc = fmt.Sprintf("A sign reads %s", s.Repo)
	}

	itemsDesc := ""
	if len(s.Contents.Files) > 0 {
		itemsDesc = "there are pieces of paper strewn about the floor."
	}

	err = tmpl.Execute(out, struct {
		RoomDescription string
		RoomFlavor      string
		ItemsDesc       string
	}{
		RoomDescription: desc,
		RoomFlavor:      "TODO ROOM FLAVOR",
		ItemsDesc:       itemsDesc,
	})
	if err != nil {
		panic(err.Error())
	}

	return out.String()
}

func main() {
	// TODO detect or take argument
	repo := "cli/cli"

	io := &IOStreams{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	repl := NewIOREPL(io)
	client, err := gh.RESTClient(nil)
	if err != nil {
		fmt.Printf("could not create client: %s\n", err.Error())
		os.Exit(1)
	}
	err = _main(&Opts{
		IO:     io,
		Repo:   repo,
		Getter: NewRESTContentGetter(client, repo),
		Args:   os.Args,
		REPL:   repl,
	})
	if err != nil {
		fmt.Println(err.Error())
		os.Exit(2)
	}
}

/*

You're standing in a room. A |large sign| reads, "gh-dungeon".

|Flavor text based on dungeon hash flavor|

There is a door marked with a staircase against one wall.

? for suggestions.

> go through door

You're standing in a |hallway|. |There are doors lining the walls|.

You walk down the |hallway|, checking each door's sign.

Go through which door?

> internal
  pkg
  templates
  vendor

<enter>

You |pull open the door| marked "internal". Before you is a |spiral staircase|.

Heading down, you emerge into a new room. A |scrawl on the floor| reads, "internal".

|Flavor text based on dungeon hash flavor|

There is a door marked with a staircase against one wall.

> look

You see objects scattered about the room:

- a |large gemstone| labeled "api.go"
- a |rolled up scroll| labeled "foo.txt"
- a |stone tablet| labeled "bar.go"
- a |blinking data pad| labeled "baz.go"

> read api.go

You pick up the |large gemstone|. |Words are projected from its depths into the air in front of you|.

(Press enter to view "api.go")

<enter> <pager opens>
package api

...

<q>

You are standing in a room. A |scrawl on the floor| reads, "internal".

|Flavor text based on dungeon hash flavor|

There is a door marked with a staircase against one wall.

>

*/
