package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"
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
	SHA   string
}

type ContentGetter interface {
	GetDirContents(path string, ref string) (Contents, error)
	GetFileContents(path string, ref string) (FileResponse, error)
	GetPrevSHA(path, ref string) (sha string, err error)
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

type CommitsResponse struct {
	SHA string `json:"sha"`
}

func (g *RESTContentGetter) GetPrevSHA(path, ref string) (sha string, err error) {
	vs := url.Values{}
	vs.Set("path", path)
	if sha != "" {
		vs.Set("sha", sha)
	}
	u := fmt.Sprintf("repos/%s/commits?%s", g.repo, vs.Encode())
	var resp []CommitsResponse
	err = g.client.Get(u, &resp)
	if err != nil {
		return
	}

	if len(resp) < 1 {
		// TODO custom error type
		err = errors.New("no previous commit")
		return
	}

	sha = resp[1].SHA

	return
}

type FileResponse struct {
	Type string
	Name string
	SHA  string `json:"sha"`
}

func (g *RESTContentGetter) GetFileContents(path, ref string) (resp FileResponse, err error) {
	query := ""
	if ref != "" {
		vs := url.Values{}
		vs.Set("ref", ref)
		query = "?" + vs.Encode()
	}
	u := fmt.Sprintf("repos/%s/contents/%s%s", g.repo, path, query)
	err = g.client.Get(u, &resp)
	return
}

func (g *RESTContentGetter) GetDirContents(path, ref string) (c Contents, err error) {
	query := ""
	if ref != "" {
		vs := url.Values{}
		vs.Set("ref", ref)
		query = "?" + vs.Encode()
	}
	c = Contents{
		Files: []FileResponse{},
		Dirs:  []string{},
	}
	var resp []FileResponse
	u := fmt.Sprintf("repos/%s/contents/%s%s", g.repo, path, query)
	if err = g.client.Get(u, &resp); err != nil {
		return
	}

	// TODO handle error. if 404, kick user to "lost in time and space" scene then return to root.

	for _, entry := range resp {
		if entry.Type == "file" {
			c.Files = append(c.Files, entry)
		} else if entry.Type == "dir" {
			c.Dirs = append(c.Dirs, entry.Name)
		}
	}
	// TODO handle SHA

	return
}

func _main(opts *Opts) error {
	g := opts.Getter
	repl := opts.REPL
	out := opts.IO.Out

	state := GameState{
		Repo: opts.Repo,
	}

	var cmd Command
	var err error
	contents, err := g.GetDirContents(state.Path, "")
	if err != nil {
		fmt.Println(err)
		panic("TODO handle this")
	}
	state.Contents = contents

	fmt.Fprintln(out, state.RenderRoom())

	for {
		contents, err := g.GetDirContents(state.Path, state.SHA)
		if err != nil {
			panic("TODO handle this")
		}
		state.Contents = contents
		cmd, err = repl.NextCommand()
		if err != nil {
			if uce, ok := err.(*UnknownCommandError); ok {
				fmt.Fprintln(out, uce.Error())
				if uce.Hint != "" {
					fmt.Fprintf(out, "hint: %s\n", uce.Hint)
				}
				continue
			} else {
				break
			}
		}

		if cmd.Kind == LookCommand {
			fmt.Fprintln(out, state.RenderRoom())
		}

		if cmd.Kind == ShiftCommand {
			switch cmd.Args[0] {
			case "back":
				fmt.Fprintln(out, "you close your eyes and focus on the past. ")
				prevSHA, err := g.GetPrevSHA(state.Path, state.SHA)
				if err != nil {
					panic("TODO Deal with this")
				}

				state.SHA = prevSHA

				fmt.Fprintln(out, "you feel as though things have changed around you.")

			case "forward":
				fmt.Fprintf(out, "you close your eyes and focus on the future.")

				// TODO
				// - do we need to hit the API? if we do, how do we ask for newer commits? the API wants a commit to start from.
				// - we could keep track of past SHAs, but this won't help in this case:
				// - shift backwards
				// - descend into a directory
				// - (do we know at what sha that directory is? i suppose the contents dir listing should give us that)
				// - run shift forward - how to ask for newer commit from API?

				fmt.Fprintln(out, "you feel as though things have changed around you.")
			default:
				panic("you shouldn't be here")
			}
		}

		if cmd.Kind == GoCommand {
			switch cmd.Args[0] {
			case "up":
				if state.Path == "" {
					fmt.Fprintf(out, "you search the walls for a door out but can't find out.")
				} else {
					fmt.Fprintln(out, "you open the door and follow a spiral staircase up to a previous level.")
					state.PopPath()
				}
			case "down":
				which, err := repl.GoDown(state.Contents.Dirs)
				if err != nil {
					return err
				}
				state.PushPath(which)
			default:
				panic("should not get here, yo")
			}
		}

		if cmd.Kind == ExamineCommand {
			if len(state.Contents.Files) == 0 {
				fmt.Fprintln(out, "you don't see anything to examine in here")
			} else {
				var which FileResponse
				which, err = repl.ExamineWhich(state.Contents.Files)
				if err != nil {
					break
				}
				fmt.Fprintf(out, "you are holding a paper titled %s.", which.Name)

				// TODO do a confirm then open a pager
			}
		}

		if cmd.Kind == QuitCommand {
			fmt.Fprintln(out, "see you again~")
			break
		}

		if cmd.Kind == HelpCommand {
			fmt.Fprintln(out, "supported verbs: look, go, examine, quit")
		}
	}

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
	GoDown(dirs []string) (string, error)
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
	ShiftCommand   CommandKind = "shift"
)

func (r *IOREPL) GoDown(doors []string) (string, error) {
	out := r.io.Out
	fmt.Fprintln(out, "you open the door.")
	fmt.Fprintln(out, "before you is a dim, spiraling staircase going down.")
	fmt.Fprintln(out, "as you descend, doors emerge from the darkness at regular intervals upon small landings.")

	var selected int
	if err := survey.AskOne(&survey.Select{
		Message: "at which door will you stop?",
		Options: doors,
	}, &selected); err != nil {
		return "", err
	}

	return doors[selected], nil
}

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
		Message: ">",
	}, &raw); err != nil {
		return
	}

	return parseCommand(raw)
}

type UnknownCommandError struct {
	Raw  string
	Hint string
}

func (e *UnknownCommandError) Error() string {
	return "i did not understand :( supported verbs: look, go, examine, quit"
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
	} else if strings.HasPrefix(raw, "quit") || raw == "q" {
		cmd = Command{
			Raw:  raw,
			Kind: QuitCommand,
		}
	} else if strings.HasPrefix(raw, "go") {
		split := strings.Split(raw, " ")
		if len(split) != 2 {
			err = &UnknownCommandError{
				Raw:  raw,
				Hint: "try 'go down' or 'go up'",
			}
		} else {
			if split[1] != "down" && split[1] != "up" {
				err = &UnknownCommandError{
					Raw:  raw,
					Hint: "try 'go down' or 'go up'",
				}
			}
			cmd = Command{
				Raw:  raw,
				Kind: GoCommand,
				Args: []string{split[1]},
			}
		}
	} else if strings.HasPrefix(raw, "shift") {
		split := strings.Split(raw, " ")
		if len(split) != 2 {
			err = &UnknownCommandError{
				Raw:  raw,
				Hint: "try 'shift back' or 'shift forward'",
			}
		} else {
			if split[1] != "back" && split[1] != "forward" {
				err = &UnknownCommandError{
					Raw:  raw,
					Hint: "try 'shift back' or 'shift forward'",
				}
			}
			cmd = Command{
				Raw:  raw,
				Kind: ShiftCommand,
				Args: []string{split[1]},
			}
		}
	} else if strings.HasPrefix(raw, "?") {
		cmd = Command{
			Raw:  raw,
			Kind: HelpCommand,
		}
	} else {
		err = &UnknownCommandError{
			Raw: raw,
		}
	}

	return
}

type GameState struct {
	Repo      string
	Path      string
	Contents  Contents
	PathStack []string
	SHA       string
}

const roomTmpl string = `
You are standing in a room of plain construction. There is a drop ceiling above you with scattered flourescent lighting.

It smells of stale coffee, but you can find none to drink.

{{ .RoomDescription }}

{{ .RoomFlavor }}

{{ .DownDesc }}
{{ .UpDesc }}
{{ .ItemsDesc }}
`

func (s *GameState) ReverseSHA() {

}

func (s *GameState) ForwardSHA() {

}

func (s *GameState) PopPath() {
	s.PathStack = s.PathStack[0 : len(s.PathStack)-1]
	if len(s.PathStack) == 0 {
		s.Path = ""
	} else {
		s.Path = s.PathStack[len(s.PathStack)-1]
	}
}

func (s *GameState) PushPath(path string) {
	s.PathStack = append(s.PathStack, path)
	s.Path = s.Path + "/" + path
}

func (s *GameState) RenderRoom() string {
	tmpl, err := template.New("room").Parse(roomTmpl)
	if err != nil {
		panic(err.Error())
	}
	out := &bytes.Buffer{}
	desc := ""
	if s.Path == "" {
		desc = fmt.Sprintf("A sign reads '%s'", s.Repo)
	} else {
		desc = fmt.Sprintf("A sign reads '%s'", s.PathStack[len(s.PathStack)-1])
	}

	itemsDesc := ""
	if len(s.Contents.Files) > 0 {
		itemsDesc = "there are pieces of paper strewn about the floor."
	}

	var downDesc string
	if len(s.Contents.Dirs) > 0 {
		downDesc = "there is a door marked with a staircase and a down arrow along one wall."
	}

	var upDesc string
	if s.Path != "" {
		upDesc = "there is a door marked with a staircase and an up arrow along one wall."
	}

	err = tmpl.Execute(out, struct {
		RoomDescription string
		RoomFlavor      string
		ItemsDesc       string
		DownDesc        string
		UpDesc          string
	}{
		RoomDescription: desc,
		RoomFlavor:      "dust motes float through the air.",
		ItemsDesc:       itemsDesc,
		DownDesc:        downDesc,
		UpDesc:          upDesc,
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
