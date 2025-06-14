// Copyright 2021-2025 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package internal_test

import (
	"path/filepath"
	"testing"

	"github.com/cerbos/cerbos-sdk-go/internal"
	"github.com/jdx/go-netrc"
	"github.com/stretchr/testify/require"
)

func TestLoadBasicAuth(t *testing.T) {
	netrcPath := mkNetrc(t)
	testCases := []struct {
		name           string
		env            internal.Environment
		providedServer string
		providedUser   string
		providedPass   string
		wantErr        bool
		wantServer     string
		wantUser       string
		wantPass       string
	}{
		{
			name: "non empty provided values",
			env: mockEnv{
				internal.NetrcEnvVar:    netrcPath,
				internal.UsernameEnvVar: "envuser",
				internal.PasswordEnvVar: "envpass",
				internal.ServerEnvVar:   "envserver",
			},
			providedUser:   "user",
			providedPass:   "pass",
			providedServer: "server",
			wantUser:       "user",
			wantPass:       "pass",
			wantServer:     "server",
		},
		{
			name: "empty provided server",
			env: mockEnv{
				internal.NetrcEnvVar:    netrcPath,
				internal.UsernameEnvVar: "envuser",
				internal.PasswordEnvVar: "envpass",
				internal.ServerEnvVar:   "envserver",
			},
			providedUser:   "user",
			providedPass:   "pass",
			providedServer: "",
			wantUser:       "user",
			wantPass:       "pass",
			wantServer:     "envserver",
		},
		{
			name: "empty provided user",
			env: mockEnv{
				internal.NetrcEnvVar:    netrcPath,
				internal.UsernameEnvVar: "envuser",
				internal.PasswordEnvVar: "envpass",
				internal.ServerEnvVar:   "envserver",
			},
			providedUser:   "",
			providedPass:   "pass",
			providedServer: "server",
			wantUser:       "envuser",
			wantPass:       "pass",
			wantServer:     "server",
		},
		{
			name: "empty provided password",
			env: mockEnv{
				internal.NetrcEnvVar:    netrcPath,
				internal.UsernameEnvVar: "envuser",
				internal.PasswordEnvVar: "envpass",
				internal.ServerEnvVar:   "envserver",
			},
			providedUser:   "user",
			providedPass:   "",
			providedServer: "server",
			wantUser:       "user",
			wantPass:       "envpass",
			wantServer:     "server",
		},
		{
			name: "empty provided values",
			env: mockEnv{
				internal.NetrcEnvVar:    netrcPath,
				internal.UsernameEnvVar: "envuser",
				internal.PasswordEnvVar: "envpass",
				internal.ServerEnvVar:   "envserver",
			},
			providedUser:   "",
			providedPass:   "",
			providedServer: "",
			wantUser:       "envuser",
			wantPass:       "envpass",
			wantServer:     "envserver",
		},
		{
			name:           "netrc fallback (provided hostname)",
			env:            mockEnv{internal.NetrcEnvVar: netrcPath},
			providedUser:   "",
			providedPass:   "",
			providedServer: "server:3592",
			wantUser:       "netrcuser",
			wantPass:       "netrcpass",
			wantServer:     "server:3592",
		},
		{
			name:           "netrc fallback (env hostname)",
			env:            mockEnv{internal.NetrcEnvVar: netrcPath, internal.ServerEnvVar: "dns:///server:3592"},
			providedUser:   "",
			providedPass:   "",
			providedServer: "",
			wantUser:       "netrcuser",
			wantPass:       "netrcpass",
			wantServer:     "dns:///server:3592",
		},
		{
			name:           "no netrc entry",
			env:            mockEnv{internal.NetrcEnvVar: netrcPath, internal.ServerEnvVar: "dns:///someserver:3592"},
			providedUser:   "",
			providedPass:   "",
			providedServer: "",
			wantErr:        true,
		},
		{
			name:           "no netrc file",
			env:            mockEnv{internal.NetrcEnvVar: "test", internal.ServerEnvVar: "dns:///server:3592"},
			providedUser:   "",
			providedPass:   "",
			providedServer: "",
			wantErr:        true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			haveServer, haveUser, havePass, haveErr := internal.LoadBasicAuthData(tc.env, tc.providedServer, tc.providedUser, tc.providedPass)
			if tc.wantErr {
				require.Error(t, haveErr)
				return
			}

			require.NoError(t, haveErr)
			require.Equal(t, tc.wantServer, haveServer)
			require.Equal(t, tc.wantUser, haveUser)
			require.Equal(t, tc.wantPass, havePass)
		})
	}
}

func mkNetrc(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "netrc")

	n := netrc.New(path)
	n.AddMachine("server", "netrcuser", "netrcpass")
	n.AddMachine("192.168.1.23", "netrcuser", "netrcpass")
	require.NoError(t, n.Save())

	return path
}

func TestExtractMachineName(t *testing.T) {
	testCases := []struct {
		target  string
		want    string
		wantErr bool
	}{
		{
			target: "myserver",
			want:   "myserver",
		},
		{
			target: "myserver:3593",
			want:   "myserver",
		},
		{
			target: "dns:myserver:3593",
			want:   "myserver",
		},
		{
			target: "dns:///myserver:3593",
			want:   "myserver",
		},
		{
			target: "dns://192.168.1.1/myserver:3593",
			want:   "myserver",
		},
		{
			target: "10.0.1.2",
			want:   "10.0.1.2",
		},
		{
			target: "10.0.1.2:3593",
			want:   "10.0.1.2",
		},
		{
			target: "[::1]:80",
			want:   "::1",
		},
		{
			target: "",
			want:   "",
		},
		{
			target: ":",
			want:   "",
		},
		{
			target: "   ",
			want:   "   ",
		},
		{
			target:  "dns://myserver:3593",
			wantErr: true,
		},
		{
			target:  "unix:/path",
			wantErr: true,
		},
		{
			target:  "unix:///path",
			wantErr: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.target, func(t *testing.T) {
			have, err := internal.ExtractMachineName(tc.target)
			if tc.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, have)
		})
	}
}

type mockEnv map[string]string

func (m mockEnv) Getenv(k string) string {
	return m[k]
}

func (m mockEnv) LookupEnv(k string) (string, bool) {
	v, ok := m[k]
	return v, ok
}
