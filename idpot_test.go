package idpot

import (
  "crypto/sha1"
  "fmt"
  "io"
  "io/ioutil"
  "math/rand"
  "os"
  "os/exec"
  "runtime"
  "strings"
  "sync"
  "syscall"
  "testing"
  "time"
  "github.com/lestrrat/go-tcptest"
  "github.com/lestrrat/go-test-mysqld"
  "github.com/lestrrat/go-idpot/client"
)

func init () {
  rand.Seed(time.Now().UnixNano())
  buildCommandLineTools()

  // Create a mysqld instance for testing
}

func randomName() string {
  h := sha1.New()
  io.WriteString(h,
    fmt.Sprintf(
      "idpot.test.%d.%d",
      os.Getpid(),
      time.Now().UnixNano(),
    ),
  )

  return fmt.Sprintf("%x", h.Sum(nil))
}

func buildCommandLineTools() {
  // compile command
  cmd := exec.Command("go", "build", "-o", "bin/idpot-server", "cli/idpot-server.go")
  err := cmd.Run()
  if err != nil {
    panic(fmt.Sprintf("Failed to compile idpot-server: %s", err))
  }

  // Must add it to PATH so we can invoke it
  os.Setenv("PATH",
    strings.Join([]string {
      "bin",
      os.Getenv("PATH"),
    }, ":",),
  )
}

func TestServer(t *testing.T) {
  mysqld, err := mysqltest.NewMysqld(nil)
  if err != nil {
    t.Fatalf("Failed to start mysqld: %s", err)
  }
  defer mysqld.Stop()

  fullpath, err := exec.LookPath("idpot-server")
  if err != nil {
    t.Skipf("Could not find idpot-server in PATH")
  }

  // Create a config file to be used by idpot-server
  dir, err := ioutil.TempDir("", "idpot-test")
  if err != nil {
    t.Fatalf("Failed to create temporary directory: %s", err)
  }
  defer os.RemoveAll(dir)

  cfgfile, err := ioutil.TempFile(dir, "idpot.gcfg")
  if err != nil {
    t.Fatalf("Failed to create temp file: %s", err)
  }

  io.WriteString(cfgfile, fmt.Sprintf(
    `
[Server]
LogFile=idpot-%%Y%%m%%d-access_log
LogLinkName=idpot-access_log

[Mysql]
ConnectString=%s
`,
    mysqld.Datasource("test", "", "", 0),
  ))
  cfgfile.Close()

  var cmd *exec.Cmd
  start := func(port int) {
    numprocs := runtime.GOMAXPROCS(-1)
    os.Setenv("GOMAXPROCS", fmt.Sprintf("%d", numprocs))
    cmd = exec.Command(
      fullpath,
      "--listen", fmt.Sprintf("127.0.0.1:%d", port),
      "--config", cfgfile.Name(),
    )
    t.Logf("Starting command %v", cmd.Args)
    cmd.SysProcAttr = &syscall.SysProcAttr {
      Setpgid: true,
    }
    out, _ := cmd.CombinedOutput()
    t.Logf(string(out))
  }

  // Launch server in separate process
  server, err := tcptest.Start(start, 30 * time.Second)
  if err != nil {
    t.Fatalf("Failed to start idpot-server: %s", err)
  }

  port := server.Port()

  // First, declare a new pot
  base   := fmt.Sprintf("http://127.0.0.1:%d", port)
  client := client.New(base)

  for _, min := range []uint64 { 0, 100, 1000, 10000 } {
    pot    := randomName()
    err     = client.CreatePot(pot, min)
    if err != nil {
      t.Fatalf("Failed to create new pot %s: %s", pot, err)
    }

    t.Logf("Created new pot %s", pot)

    sem := make(chan bool, 1)

    wg := &sync.WaitGroup {}
    ids := make(map[uint64]bool)

    maxGoros := 10
    maxFetches := 10000

    t0 := time.Now()
    for i := 0; i < maxGoros; i++ {
      wg.Add(1)
      go func() {
        defer func() { wg.Done() }()
        for j := 0; j < maxFetches; j++ {
          id, err := client.NextId(pot)
          if err != nil {
            t.Errorf("Failed to fetch id: %s", err)
            continue
          }

          sem<- true
          if _, ok := ids[id]; ok {
            // collision!
            t.Errorf("Collision detected for id %d :/", id)
          } else {
            ids[id] = true
          }
          <-sem
        }
      }()
    }

    wg.Wait()
    elapsed := time.Since(t0)

    // I should have 1000 ids
    count := 0
    for v, _ := range ids {
      if v < min {
        t.Errorf("Id given is out of range: %d (wanted > %d)", v, min)
      }
      count++
    }

    if count != maxGoros * maxFetches {
      t.Errorf("Did not get %d ids, got %d", maxGoros * maxFetches, count)
    }

    t.Logf("Fetched %d ids in %f secs (%f fetches/sec)", count, elapsed.Seconds(), float64(count) / elapsed.Seconds())
  }

  cmd.Process.Signal(syscall.SIGTERM)
  server.Wait()
}