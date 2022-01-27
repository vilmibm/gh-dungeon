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
	// TODO
}

type ContentGetter interface {
	GetPathContents(path string) (Contents, error)
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

func (g *RESTContentGetter) GetPathContents(path string) (Contents, error) {
	// TODO
	return Contents{}, nil
}

func _main(opts *Opts) error {
	g := opts.Getter
	repl := opts.REPL

	state := GameState{}

	contents, err := g.GetPathContents("")
	if err != nil {
		return err
	}
	fmt.Printf("DBG %#v\n", contents)

	var cmd Command

	for {
		fmt.Fprintln(opts.IO.Out, state.RenderRoom())
		cmd, err = repl.NextCommand()
		if err != nil && errors.Is(err, UnknownCommandError{}) {
			fmt.Fprintln(opts.IO.Out, err.Error())
			err = nil
			continue
		} else if err != nil {
			break
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
	LookCommand CommandKind = "look"
	GoCommand   CommandKind = "go"
	QuitCommand CommandKind = "quit"
	HelpCommand CommandKind = "help"
)

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
	Path string
}

const roomTmpl string = `
You are standing in a room. {{ .RoomDescription }}

{{ .RoomFlavor }}
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
	err = tmpl.Execute(out, struct {
		RoomDescription string
		RoomFlavor      string
	}{
		RoomDescription: "TODO ROOM DESC",
		RoomFlavor:      "TODO ROOM FLAVOR",
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
