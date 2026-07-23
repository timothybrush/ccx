package utils

import (
	"bytes"
	"compress/gzip"
	"compress/zlib"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/BenedictKing/ccx/internal/errutil"
	"github.com/klauspost/compress/zstd"
)

// DecompressGzipIfNeeded 检测并解压缩压缩响应体。
// 保留旧函数名兼容现有调用；实际支持 gzip / deflate / zstd。
func DecompressGzipIfNeeded(resp *http.Response, bodyBytes []byte) []byte {
	decompressed, err := DecompressResponseBytesIfNeeded(resp, bodyBytes)
	if err != nil {
		log.Printf("[Compression] 警告: 解压缩响应体失败: %v", err)
		return bodyBytes
	}
	return decompressed
}

// DecompressResponseBytesIfNeeded 根据 Content-Encoding 解压完整响应体。
func DecompressResponseBytesIfNeeded(resp *http.Response, bodyBytes []byte) ([]byte, error) {
	if resp == nil {
		return bodyBytes, nil
	}
	return DecompressBytesByEncoding(bodyBytes, resp.Header.Get("Content-Encoding"))
}

// DecompressBytesByEncoding 根据指定 Content-Encoding 解压完整字节数组。
func DecompressBytesByEncoding(bodyBytes []byte, encoding string) ([]byte, error) {
	reader, err := NewDecompressedReaderByEncoding(encoding, io.NopCloser(bytes.NewReader(bodyBytes)))
	if err != nil {
		return bodyBytes, err
	}
	defer errutil.IgnoreDeferred(reader.Close)
	return io.ReadAll(reader)
}

// DecompressResponseBodyIfNeeded 将 resp.Body 包装为按 Content-Encoding 解压后的 reader。
func DecompressResponseBodyIfNeeded(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	reader, err := NewDecompressedReader(resp, resp.Body)
	if err != nil {
		return err
	}
	resp.Body = reader
	return nil
}

// NewDecompressedReader 根据响应头创建解压 reader；identity 保持原样，未知编码返回错误。
func NewDecompressedReader(resp *http.Response, body io.ReadCloser) (io.ReadCloser, error) {
	if resp == nil || body == nil {
		return body, nil
	}
	return NewDecompressedReaderByEncoding(resp.Header.Get("Content-Encoding"), body)
}

// NewDecompressedReaderByEncoding 根据指定编码创建解压 reader；identity 保持原样，未知编码返回错误。
func NewDecompressedReaderByEncoding(encoding string, body io.ReadCloser) (io.ReadCloser, error) {
	if body == nil {
		return nil, nil
	}
	encoding = strings.ToLower(strings.TrimSpace(strings.Split(encoding, ",")[0]))
	switch encoding {
	case "", "identity":
		return body, nil
	case "gzip":
		reader, err := gzip.NewReader(body)
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		return &wrappedReadCloser{Reader: reader, close: func() error {
			err1 := reader.Close()
			err2 := body.Close()
			if err1 != nil {
				return err1
			}
			return err2
		}}, nil
	case "deflate":
		reader, err := zlib.NewReader(body)
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		return &wrappedReadCloser{Reader: reader, close: func() error {
			err1 := reader.Close()
			err2 := body.Close()
			if err1 != nil {
				return err1
			}
			return err2
		}}, nil
	case "zstd":
		reader, err := zstd.NewReader(body)
		if err != nil {
			_ = body.Close()
			return nil, err
		}
		return &wrappedReadCloser{Reader: reader, close: func() error {
			reader.Close()
			return body.Close()
		}}, nil
	default:
		_ = body.Close()
		return nil, fmt.Errorf("unsupported content-encoding: %s", encoding)
	}
}

type wrappedReadCloser struct {
	io.Reader
	close func() error
}

func (r *wrappedReadCloser) Close() error {
	if r.close == nil {
		return nil
	}
	return r.close()
}
