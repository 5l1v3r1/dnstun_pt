// Dummy no-op pluggable transport server. Works only as a managed proxy.
//
// Usage (in torrc):
//  BridgeRelay 1
//  ORPort 9001
//  ExtORPort 6669
//  ServerTransportPlugin dummy exec dummy-server
//
// Because the dummy transport doesn't do anything to the traffic, you can
// connect to it with any ordinary Tor client; you don't have to use
// dummy-client.
package main

import (
  "net"
  "os"
  "os/exec"
  "os/signal"
  "syscall"
)

import "git.torproject.org/pluggable-transports/goptlib.git"

var logfile *os.File

var ptInfo pt.ServerInfo

// When a connection handler starts, +1 is written to this channel; when it
// ends, -1 is written.
var handlerChan = make(chan int)

func copyLoop(a, b net.Conn) {
  // a = 127.0.0.1:5353 (server)
  // b = 127.0.0.1:54861 (randomport)
  logfile.WriteString("server\n")
  logfile.WriteString(a.LocalAddr().String())
  logfile.WriteString("\n")
  logfile.WriteString(b.LocalAddr().String())
  logfile.WriteString("\n")
  logfile.WriteString(a.RemoteAddr().String())
  logfile.WriteString("\n")
  logfile.WriteString(b.RemoteAddr().String())
  logfile.WriteString("\n")
  logfile.WriteString("server\n")

  // CMD STUFF
  cmd := exec.Command("rvmsudo", 
    "ruby", "/home/ec2-user/dnscat2_temp/server/dnscat2.rb", "simpleapp.me", "-u")

  cmd.Stdin = b 
  cmd.Stderr = b

  err2 := cmd.Start()
  if err2 != nil {
    logfile.WriteString(err2.Error())
  }

  logfile.WriteString("done\n")

  cmd.Wait()
}

func handler(conn net.Conn) error {
  defer conn.Close()

  logfile.WriteString("handler\n")

  handlerChan <- 1
  defer func() {
    handlerChan <- -1
  }()

  or, err := pt.DialOr(&ptInfo, conn.RemoteAddr().String(), "dummy")
  if err != nil {
    return err
  }
  defer or.Close()

  copyLoop(conn, or)

  return nil
}

func acceptLoop(ln net.Listener) error {
  logfile.WriteString("accept loop\n")
  defer ln.Close()
  for {
    logfile.WriteString("before accept\n")
    conn, err := ln.Accept()
    logfile.WriteString("after accept\n")
    if err != nil {
      if e, ok := err.(net.Error); ok && !e.Temporary() {
        return err
      }
      continue
    }
    go handler(conn)
  }
}

func main() {
  logfile, _ = os.Create("/Users/irvinzhan/Documents/open-source/tor/goptlib/examples/dummy-client/logs/server.log")
  defer logfile.Close()

  logfile.WriteString("main\n")

  var err error

  ptInfo, err = pt.ServerSetup([]string{"dummy"})
  if err != nil {
    os.Exit(1)
  }

  listeners := make([]net.Listener, 0)
  for _, bindaddr := range ptInfo.Bindaddrs {
    switch bindaddr.MethodName {
    case "dummy":
      logfile.WriteString(bindaddr.Addr.String())
      ln, err := net.ListenTCP("tcp", bindaddr.Addr)
      if err != nil {
        pt.SmethodError(bindaddr.MethodName, err.Error())
        break
      }
      go acceptLoop(ln)
      pt.Smethod(bindaddr.MethodName, ln.Addr())
      listeners = append(listeners, ln)
    default:
      pt.SmethodError(bindaddr.MethodName, "no such method")
    }
  }
  pt.SmethodsDone()

  var numHandlers int = 0
  var sig os.Signal
  sigChan := make(chan os.Signal, 1)
  signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

  // wait for first signal
  sig = nil
  for sig == nil {
    select {
    case n := <-handlerChan:
      numHandlers += n
    case sig = <-sigChan:
    }
  }
  for _, ln := range listeners {
    ln.Close()
  }

  if sig == syscall.SIGTERM {
    return
  }

  // wait for second signal or no more handlers
  sig = nil
  for sig == nil && numHandlers != 0 {
    select {
    case n := <-handlerChan:
      numHandlers += n
    case sig = <-sigChan:
    }
  }


}
