package internal

import (
	"bytes"
	"context"
	"testing"
)

type fakeProvider struct {
	secret Secret
}

func (f fakeProvider) Get(context.Context, string, string) (Secret, error) {
	return f.secret, nil
}

func TestResolverDeriveIsDeterministicAndScoped(t *testing.T) {
	root := bytes.Repeat([]byte{1}, keySize)
	resolver, err := NewResolver(fakeProvider{secret: Secret{Value: root, Version: "1"}}, "root", "1", "prod/order")
	if err != nil {
		t.Fatal(err)
	}
	first, err := resolver.Derive(context.Background(), "jwt")
	if err != nil {
		t.Fatal(err)
	}
	second, err := resolver.Derive(context.Background(), "jwt")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("same derivation inputs produced different keys")
	}
	different, err := resolver.Derive(context.Background(), "mfa")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, different) {
		t.Fatal("different purposes produced the same key")
	}
}
