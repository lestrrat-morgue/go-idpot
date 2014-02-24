package server

import (
  "database/sql"
  "errors"
  "fmt"
  "io"
  "log"
  "net"
  "net/http"
  "strconv"
  "github.com/braintree/manners"
  "github.com/lestrrat/go-file-rotatelogs"
  "github.com/lestrrat/go-server-starter-listener"
  "github.com/lestrrat/go-apache-logformat"
  _ "github.com/go-sql-driver/mysql"
)

type MysqlServer struct {
  ConnectString string
  Hostname  string
  Port      uint32
  Username  string
  Password  string
  Dbname    string
}

func (s *MysqlServer) FormatDSN() string {
  if s.ConnectString != "" {
    return s.ConnectString
  }

  return fmt.Sprintf(
    "%s:%s@tcp(%s:%d)/%s?charset=utf8mb4,utf8",
    s.Username,
    s.Password,
    s.Hostname,
    s.Port,
    s.Dbname,
  )
}

func (s *MysqlServer) Connect() (*sql.DB, error) {
  dsn := s.FormatDSN()
  db, err := sql.Open("mysql", dsn)
  if err != nil {
    return nil, errors.New(
      fmt.Sprintf(
        "Error connecting to mysql database: %s",
        err,
      ),
    )
  }
  return db, nil
}

type Server struct {
  listen      string
  logger      *apachelog.ApacheLog
  rotatelogs  *rotatelogs.RotateLogs
  dbserver    *MysqlServer
}

type ServerOpts struct {
  Listen string
  LogFile string
  LogLinkName string
  Mysql *MysqlServer
}

func New(opts *ServerOpts) (*Server) {
  dbserver := &MysqlServer {
    Hostname: "127.0.0.1",
    Port:     3306,
    Username: "root",
    Password: "",
    Dbname:   "idpot",
  }
  listen := "0.0.0.0:8080"
  logger := apachelog.CombinedLog.Clone()
  var rl *rotatelogs.RotateLogs
  if opts != nil  {
    if opts.LogFile != "" {
      rl = rotatelogs.NewRotateLogs(opts.LogFile)
      if opts.LogLinkName != "" {
        rl.LinkName = opts.LogLinkName
      }
      logger.SetOutput(rl)
    }

    if opts.Listen != "" {
      listen = opts.Listen
    }

    if cfg := opts.Mysql ; cfg != nil {
      if cfg.ConnectString != "" {
        dbserver.ConnectString = cfg.ConnectString
      }
      if cfg.Hostname != "" {
        dbserver.Hostname = cfg.Hostname
      }
      if cfg.Port > 0 {
        dbserver.Port = cfg.Port
      }
      if cfg.Username != "" {
        dbserver.Username = cfg.Username
      }
      if cfg.Password != "" {
        dbserver.Password = cfg.Password
      }
      if cfg.Dbname != "" {
        dbserver.Dbname = cfg.Dbname
      }
    }
  }

  return &Server {
    logger:     logger,
    rotatelogs: rl,
    listen:     listen,
    dbserver:   dbserver,
  }
}

func (s *Server) Start() {
  defer func() {
    if rl := s.rotatelogs; rl != nil {
      rl.Close()
    }
  }()

  // Work with Server::Starter
  var baseListener net.Listener
  var err error
  if portmap, err := ss.Ports(); err == nil && len(portmap) > 0 {
    pm := portmap[0]

    log.Printf("Found Server::Starter env var. Attempting to listen on %d", pm)
    baseListener, err = ss.NewListenerOn(pm)
    if err != nil {
      log.Printf("Failed to listen to start_server fd: %s", err)
      log.Printf("Continuing on to listen on regular address")
    }
  }

  if baseListener == nil {
    log.Printf("Attempting to listen on %s", s.listen)
    baseListener, err = net.Listen("tcp", s.listen)
    if err != nil {
      log.Fatalf("Failed to listen at %s: %s", s.listen, err)
    }
  }

  m := manners.NewServer()

  l := manners.NewListener(baseListener, m)
  err = m.Serve(l, s)
  if err != nil {
    log.Fatalf("Error while trying to Serve: %s", err)
  }
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
  // Wrap w so that we can capture header's sent and stuff
  lw := apachelog.NewLoggingWriter(w, r, s.logger)
  defer lw.EmitLog()

  path := r.URL.Path
  switch {
  case len(path) > 4 && path[0:4] == "/id/":
    pot := path[4:]
    s.ServeNextIdFromPot(lw, r, pot)
  case path == "/pot/create":
    s.ServeCreatePot(lw, r)
  case len(path) > 5 && path[0:5] == "/pot/":
    pot := path[5:]
    s.ServeCheckPot(lw, r, pot)
  default:
    http.Error(lw, http.StatusText(404), 404)
  }
}

func (s *Server) ErrorResponse(w http.ResponseWriter, code int, msg string) {
  log.Printf(msg)
  http.Error(w, http.StatusText(code), code)
  io.WriteString(w, msg)
}

func (s *Server) ServeCheckPot(w http.ResponseWriter, r *http.Request, pot string) {
  db, err := s.dbserver.Connect()
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to connect to mysql server: %s", err))
    return
  }
  defer db.Close()

  table := fmt.Sprintf("pot_%s", pot)
  sql := fmt.Sprintf(`SELECT 1 FROM %s`, table)
  var ok uint32

  err = db.QueryRow(sql).Scan(&ok)
  if err != nil {
    /* Not quite it, but PROBABLY... this pot does not exists */
    s.ErrorResponse(w, 404, "Specified pot does not exist")
    return
  }

  w.WriteHeader(204)
  fmt.Fprintf(w, "%s", "Specified pot exists")
}

func (s *Server) ServeNextIdFromPot(w http.ResponseWriter, r *http.Request, pot string) {
  db, err := s.dbserver.Connect()
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to connect to mysql server: %s", err))
    return
  }
  defer db.Close()

  table := fmt.Sprintf("pot_%s", pot)

  // UPDATE, then get LAST_INSERT_ID()
  updateSQL := fmt.Sprintf(`UPDATE %s SET id = LAST_INSERT_ID(id + 1)`, table)
  _, err = db.Exec(updateSQL)
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to update table %s: %s", table, err))
    return
  }

  fetchSQL := `SELECT LAST_INSERT_ID()`
  var id uint64
  err = db.QueryRow(fetchSQL).Scan(&id)
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to fetch last insert id for table %s: %s", table, err))
    return
  }

  fmt.Fprintf(w, "%d", id)
}

func (s *Server) ServeCreatePot(w http.ResponseWriter, r *http.Request) {
  r.ParseForm()
  name := r.PostForm.Get("name")
  if name == "" {
    s.ErrorResponse(w, 500, "Required parameter 'name' not provided")
    return
  }

  var min uint64
  minStr := r.PostForm.Get("min")
  if x, err := strconv.ParseUint(minStr, 10, 64); err == nil {
    min = x
  }

  if min < 0 {
    msg := fmt.Sprintf("Parameter 'min' is out of range: %d", min)
    log.Printf(msg)
    http.Error(w, http.StatusText(500), 500)
    io.WriteString(w, msg)
    return
  }

  db, err := s.dbserver.Connect()
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to connect to mysql server: %s", err))
    return
  }
  defer db.Close()

  table := fmt.Sprintf("pot_%s", name)

  var gotLock uint32
  err = db.QueryRow(`SELECT GET_LOCK(?, 30)`, table).Scan(&gotLock)
  if err != nil || gotLock != 1 {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to acquire lock for table %s", table))
    return
  }
  lockReleased := false
  releaseLock := func() {
    if lockReleased {
      return
    }
    db.Exec(`SELECT RELEASE_LOCK(?)`, table)
  }

  defer releaseLock()

  createSQL := fmt.Sprintf(`CREATE TABLE %s (id BIGINT UNSIGNED NOT NULL) ENGINE=MyISAM`, table)
  _, err = db.Exec(createSQL)
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to create table %s: %s", table, err))
    return
  }

  insertSQL := fmt.Sprintf(`INSERT INTO %s (id) VALUES (?)`, table)
  _, err = db.Exec(insertSQL, min)
  if err != nil {
    s.ErrorResponse(w, 500, fmt.Sprintf("Failed to insert %s: %s", table, err))
    // Fuck, drop the table
    db.Exec(fmt.Sprintf(`DROP TABLE %s`, table))
    return
  }

  releaseLock()

  w.WriteHeader(201)
  io.WriteString(w, "Created new pot")
}