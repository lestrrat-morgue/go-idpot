package idpot

import (
  "log"
  "github.com/lestrrat/go-idpot/client"
)

func ExampleClient() {
  c := client.New("http://yourserver")
  err := c.CreatePot("newpot", 0)
  if err != nil {
    log.Fatalf("Failed to create new pot: %s", err)
  }

  for i := 0; i < 100; i++ {
    id, err := c.NextId("newpot")
    if err != nil {
      log.Fatalf("Failed to get a new ID: %s", err)
    }

    log.Printf("New ID: %d", id)
  }
}