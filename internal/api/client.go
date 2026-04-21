package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"

	"golang.org/x/term"
)

type Client struct {
	socketPath string
}

func NewClient(socketPath string) *Client {
	return &Client{socketPath: socketPath}
}

func (c *Client) Request(_ context.Context, req Request) (Response, error) {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return Response{}, fmt.Errorf("dial daemon: %w", err)
	}
	defer conn.Close()

	if err := writeRequest(conn, req); err != nil {
		return Response{}, err
	}

	reader := bufio.NewReader(conn)
	resp, err := readResponse(reader)
	if err != nil {
		return Response{}, err
	}
	if !resp.OK {
		return Response{}, fmt.Errorf(resp.Error)
	}
	return resp, nil
}

func (c *Client) StartInteractiveSession(_ context.Context, shellPath, cwd string) error {
	conn, err := net.Dial("unix", c.socketPath)
	if err != nil {
		return fmt.Errorf("dial daemon: %w", err)
	}
	defer conn.Close()

	cols, rows, _ := term.GetSize(int(os.Stdout.Fd()))
	req := Request{
		Action: ActionStartSession,
		Shell:  shellPath,
		CWD:    cwd,
		Rows:   rows,
		Cols:   cols,
		Attach: true,
	}
	if err := writeRequest(conn, req); err != nil {
		return err
	}

	reader := bufio.NewReader(conn)
	resp, err := readResponse(reader)
	if err != nil {
		return err
	}
	if !resp.OK {
		return fmt.Errorf(resp.Error)
	}

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err == nil {
		defer term.Restore(int(os.Stdin.Fd()), oldState)
	}

	copyDone := make(chan error, 2)
	go func() {
		_, err := io.Copy(conn, os.Stdin)
		copyDone <- err
	}()
	go func() {
		_, err := io.Copy(os.Stdout, reader)
		copyDone <- err
	}()

	err = <-copyDone
	if err == io.EOF {
		return nil
	}
	return err
}

func writeRequest(w io.Writer, req Request) error {
	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	if _, err := w.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write request: %w", err)
	}
	return nil
}

func readResponse(r *bufio.Reader) (Response, error) {
	line, err := r.ReadBytes('\n')
	if err != nil {
		return Response{}, fmt.Errorf("read response: %w", err)
	}
	var resp Response
	if err := json.Unmarshal(line, &resp); err != nil {
		return Response{}, fmt.Errorf("decode response: %w", err)
	}
	return resp, nil
}
