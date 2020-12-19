// quack is a small program meant to be statically linked and shoved into a container,
// wherein it "quacks" when run.  Its entire purpose is to sanity check container execution is working.
package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	bs, _ := json.Marshal(os.Args)
	fmt.Printf("quack! %v\n", string(bs))
}
