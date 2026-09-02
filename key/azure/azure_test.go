package azure

import (
	"context"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
)

type fakeClient struct {
	name    string
	version string
}

func (f *fakeClient) GetSecret(_ context.Context, name string, version string, _ *azsecrets.GetSecretOptions) (azsecrets.GetSecretResponse, error) {
	f.name = name
	f.version = version
	id := azsecrets.ID("https://example.vault.azure.net/secrets/" + name + "/version-1")
	value := "root"
	return azsecrets.GetSecretResponse{SecretBundle: azsecrets.SecretBundle{ID: &id, Value: &value}}, nil
}

func TestProviderGet(t *testing.T) {
	client := &fakeClient{}
	provider := newProvider(client)
	secret, err := provider.Get(context.Background(), "root", "version-1")
	if err != nil {
		t.Fatal(err)
	}
	if string(secret.Value) != "root" || secret.Version != "version-1" {
		t.Fatalf("unexpected secret: %+v", secret)
	}
	if client.name != "root" || client.version != "version-1" {
		t.Fatalf("unexpected request: %s/%s", client.name, client.version)
	}
}
