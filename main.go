package main

import (
	"fmt"
	"io"
	"os"

	"github.com/cli/go-gh"
	"github.com/cli/go-gh/pkg/api"
)

type IOStreams struct {
	In  io.Reader
	Out io.Writer
}

type Opts struct {
	Args   []string
	IO     *IOStreams
	Client api.RESTClient
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
	c := opts.Client

	// TODO detect or take argument
	repo := "cli/cli"

	getter := NewRESTContentGetter(c, repo)

	contents, err := getter.GetPathContents("")
	if err != nil {
		return err
	}
	fmt.Printf("DBG %#v\n", contents)

	// TODO write REPL
	// TODO fill in Contents struct as needed
	// TODO hash functions for repo names and file names

	return nil
}

func main() {
	io := &IOStreams{
		In:  os.Stdin,
		Out: os.Stdout,
	}
	client, err := gh.RESTClient(nil)
	if err != nil {
		fmt.Printf("could not create client: %s\n", err.Error())
		os.Exit(1)
	}
	err = _main(&Opts{
		IO:     io,
		Client: client,
		Args:   os.Args,
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
