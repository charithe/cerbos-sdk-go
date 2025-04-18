// Copyright 2021-2025 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package internal

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"unicode"

	policyv1 "github.com/cerbos/cerbos/api/genpb/cerbos/policy/v1"
	schemav1 "github.com/cerbos/cerbos/api/genpb/cerbos/schema/v1"
	"github.com/ghodss/yaml"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const (
	bufSize     = 1024 * 4        // 4KiB
	maxFileSize = 1024 * 1024 * 4 // 4MiB
	newline     = '\n'
)

var (
	jsonStart           = []byte("{")
	yamlSep             = []byte("---")
	yamlComment         = []byte("#")
	ErrMultipleYAMLDocs = errors.New("more than one YAML document detected")
)

func ReadPolicyFromFile(fsys fs.FS, path string) (*policyv1.Policy, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}

	defer f.Close()

	return ReadPolicy(f)
}

// ReadPolicy reads a policy from the given reader.
func ReadPolicy(src io.Reader) (*policyv1.Policy, error) {
	policy := &policyv1.Policy{}
	if err := ReadJSONOrYAML(src, policy); err != nil {
		return nil, err
	}

	return policy, nil
}

func ReadJSONOrYAML(src io.Reader, dest proto.Message) error {
	d := mkDecoder(io.LimitReader(src, maxFileSize))
	return d.decode(dest)
}

func mkDecoder(src io.Reader) decoder {
	buf := bufio.NewReaderSize(src, bufSize)
	prelude, _ := buf.Peek(bufSize)
	trimmed := bytes.TrimLeftFunc(prelude, unicode.IsSpace)

	if bytes.HasPrefix(trimmed, jsonStart) {
		return newJSONDecoder(buf)
	}

	return newYAMLDecoder(buf)
}

type decoder interface {
	decode(dest proto.Message) error
}

type decoderFunc func(dest proto.Message) error

func (df decoderFunc) decode(dest proto.Message) error {
	return df(dest)
}

func newJSONDecoder(src *bufio.Reader) decoderFunc {
	return func(dest proto.Message) error {
		jsonBytes, err := io.ReadAll(src)
		if err != nil {
			return err
		}

		if err := protojson.Unmarshal(jsonBytes, dest); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		return nil
	}
}

func newYAMLDecoder(src *bufio.Reader) decoderFunc {
	return func(dest proto.Message) error {
		buf := new(bytes.Buffer)
		numDocs := 0

		s := bufio.NewScanner(src)
		seenContent := false
		for s.Scan() {
			line := s.Bytes()
			trimmedLine := bytes.TrimSpace(line)

			// ignore comments
			if bytes.HasPrefix(trimmedLine, yamlComment) {
				continue
			}

			// ignore empty lines at the beginning of the file
			if !seenContent && len(trimmedLine) == 0 {
				continue
			}
			seenContent = true

			if bytes.HasPrefix(line, yamlSep) {
				numDocs++
				if numDocs > 1 || (numDocs == 1 && buf.Len() > 0) {
					return ErrMultipleYAMLDocs
				}
			}

			if _, err := buf.Write(line); err != nil {
				return fmt.Errorf("failed to buffer YAML data: %w", err)
			}
			_ = buf.WriteByte(newline)
		}

		if err := s.Err(); err != nil {
			return fmt.Errorf("failed to read from source: %w", err)
		}

		jsonBytes, err := yaml.YAMLToJSON(buf.Bytes())
		if err != nil {
			return fmt.Errorf("failed to convert YAML to JSON: %w", err)
		}

		if err := protojson.Unmarshal(jsonBytes, dest); err != nil {
			return fmt.Errorf("failed to unmarshal JSON: %w", err)
		}
		return nil
	}
}

func ReadSchemaFromFile(fsys fs.FS, path string) (*schemav1.Schema, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s: %w", path, err)
	}

	defer f.Close()
	return ReadSchema(f, path)
}

func ReadSchema(src io.Reader, id string) (*schemav1.Schema, error) {
	def, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("failed to read all bytes from reader: %w", err)
	}

	return &schemav1.Schema{
		Id:         id,
		Definition: def,
	}, nil
}
