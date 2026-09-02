package google

import (
	"context"
	"testing"

	secretmanagerpb "cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
)

type fakeClient struct {
	request *secretmanagerpb.AccessSecretVersionRequest
}

func (f *fakeClient) AccessSecretVersion(_ context.Context, request *secretmanagerpb.AccessSecretVersionRequest, _ ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	f.request = request
	return &secretmanagerpb.AccessSecretVersionResponse{Name: request.Name, Payload: &secretmanagerpb.SecretPayload{Data: []byte("root")}}, nil
}

func TestProviderGet(t *testing.T) {
	client := &fakeClient{}
	provider := newProvider(client)
	secret, err := provider.Get(context.Background(), "projects/p/secrets/root", "3")
	if err != nil {
		t.Fatal(err)
	}
	if string(secret.Value) != "root" || secret.Version != "3" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	if client.request.Name != "projects/p/secrets/root/versions/3" {
		t.Fatalf("request = %+v", client.request)
	}

	name, err := versionName("projects/p/secrets/root", "3")
	if err != nil {
		t.Fatal(err)
	}
	if name != "projects/p/secrets/root/versions/3" {
		t.Fatalf("name = %q", name)
	}
}
