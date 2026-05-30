// Package iox 提供 I/O 操作的工具函数.
package iox

import (
	"bufio"
	"errors"
	"io"
)

// ReadAll 读取 r 中的全部内容并返回字节切片.
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// ReadString 读取 r 中的全部内容并转换为字符串.
func ReadString(r io.Reader) (string, error) {
	b, err := io.ReadAll(r)
	return string(b), err
}

// ReadLines 按行读取 r 的内容，自动处理 \r\n 和 \n.
// 返回的每一行均不含行尾换行符.
func ReadLines(r io.Reader) ([]string, error) {
	var lines []string
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

// Drain 丢弃 r 中的全部内容，返回丢弃的字节数.
func Drain(r io.Reader) (int64, error) {
	return io.Copy(io.Discard, r)
}

// WriteString 将字符串 s 写入 w.
func WriteString(w io.Writer, s string) (int, error) {
	return io.WriteString(w, s)
}

// multiCloser 依次关闭多个 Closer 并收集所有错误.
type multiCloser struct {
	closers []io.Closer
}

// MultiCloser 返回一个 io.Closer，关闭时依次调用所有 closers.
// 所有关闭错误将通过 errors.Join 合并后返回.
func MultiCloser(closers ...io.Closer) io.Closer {
	return &multiCloser{closers: closers}
}

func (m *multiCloser) Close() error {
	errs := make([]error, 0, len(m.closers))
	for _, c := range m.closers {
		if err := c.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// CloseAndLog 关闭 c，若发生错误则调用 log 记录.
func CloseAndLog(c io.Closer, log func(error)) {
	if err := c.Close(); err != nil {
		log(err)
	}
}

// limitReadCloser 包装 io.ReadCloser，超出字节限制时返回错误.
type limitReadCloser struct {
	rc    io.ReadCloser
	limit int64
	read  int64
}

// LimitReadCloser 返回一个 io.ReadCloser，当读取字节数超过 n 时返回错误.
func LimitReadCloser(r io.ReadCloser, n int64) io.ReadCloser {
	return &limitReadCloser{rc: r, limit: n}
}

func (l *limitReadCloser) Read(p []byte) (int, error) {
	if l.read >= l.limit {
		// 返回 io.EOF 而非自定义错误，符合 io.Reader 约定，
		// 调用方如 io.ReadAll能正确识别读取结束
		return 0, io.EOF
	}
	remaining := l.limit - l.read
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := l.rc.Read(p)
	l.read += int64(n)
	if l.read >= l.limit && err == nil {
		// 已达限制，返回 io.EOF 表示正常结束
		err = io.EOF
	}
	return n, err
}

func (l *limitReadCloser) Close() error {
	return l.rc.Close()
}
