package utils

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/klauspost/compress/zstd"
)

type ZSTDCompressor struct {
	level zstd.EncoderLevel
}

func NewZSTDCompressor() *ZSTDCompressor {
	return &ZSTDCompressor{
		level: zstd.SpeedDefault,
	}
}

func NewZSTDCompressorWithLevel(level zstd.EncoderLevel) *ZSTDCompressor {
	return &ZSTDCompressor{
		level: level,
	}
}

func (c *ZSTDCompressor) CompressFile(srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create dest file: %w", err)
	}
	defer destFile.Close()

	if err := c.Compress(srcFile, destFile); err != nil {
		return fmt.Errorf("failed to compress: %w", err)
	}

	return nil
}

func (c *ZSTDCompressor) CompressDir(srcDir, destPath string) error {
	tarPath := destPath + ".tar"
	defer os.Remove(tarPath)

	tarFile, err := os.Create(tarPath)
	if err != nil {
		return fmt.Errorf("failed to create tar: %w", err)
	}
	defer tarFile.Close()

	tw := tar.NewWriter(tarFile)
	if err := c.writeDirToTar(srcDir, tw); err != nil {
		tw.Close()
		return fmt.Errorf("failed to write tar: %w", err)
	}
	tw.Close()
	tarFile.Close()

	tarRead, err := os.Open(tarPath)
	if err != nil {
		return fmt.Errorf("failed to open tar: %w", err)
	}
	defer tarRead.Close()

	compFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create dest: %w", err)
	}
	defer compFile.Close()

	if err := c.Compress(tarRead, compFile); err != nil {
		return fmt.Errorf("failed to compress: %w", err)
	}

	return nil
}

func (c *ZSTDCompressor) writeDirToTar(srcDir string, tw *tar.Writer) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return err
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if !info.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				return err
			}
			defer file.Close()

			if _, err := io.Copy(tw, file); err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *ZSTDCompressor) Compress(src io.Reader, dest io.Writer) error {
	encoder, err := zstd.NewWriter(dest, zstd.WithEncoderLevel(c.level))
	if err != nil {
		return fmt.Errorf("failed to create encoder: %w", err)
	}
	defer encoder.Close()

	if _, err := io.Copy(encoder, src); err != nil {
		return fmt.Errorf("failed to compress: %w", err)
	}

	return nil
}

func (c *ZSTDCompressor) Decompress(src io.Reader, dest io.Writer) error {
	decoder, err := zstd.NewReader(src)
	if err != nil {
		return fmt.Errorf("failed to create decoder: %w", err)
	}
	defer decoder.Close()

	if _, err := io.Copy(dest, decoder); err != nil {
		return fmt.Errorf("failed to decompress: %w", err)
	}

	return nil
}

func (c *ZSTDCompressor) DecompressFile(srcPath, destPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create dest file: %w", err)
	}
	defer destFile.Close()

	if err := c.Decompress(srcFile, destFile); err != nil {
		return fmt.Errorf("failed to decompress: %w", err)
	}

	return nil
}