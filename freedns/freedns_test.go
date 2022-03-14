package freedns

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

/*
func TestHttpRequest(t *testing.T) {
	_HttpRequest("GET", "http://127.0.0.1:12345", nil, nil)
}
*/

func TestGetDomainFromZone(t *testing.T) {
	assert.Equal(t, GetDomainFromZone("a.b.example.com"), "example.com")
	assert.Equal(t, GetDomainFromZone("example.com"), "example.com")
}

func TestOperations(t *testing.T) {
	freeDNS := FreeDNS{}

	var UserName = os.Getenv("FREEDNS_USERNAME")
	var Password = os.Getenv("FREEDNS_PASSWORD")
	var SelectedDomain = os.Getenv("FREEDNS_DOMAIN")

	require.NotEmpty(t, UserName, "Please set the env vars for FREEDNS_USERNAME")
	require.NotEmpty(t, Password, "Please set the env vars for FREEDNS_PASSWORD")
	require.NotEmpty(t, SelectedDomain, "Please set the env vars for FREEDNS_DOMAIN")

	require.Nil(t, freeDNS.Login(UserName, Password))
	require.Nil(t, freeDNS.SelectDomain(SelectedDomain))
	require.Nil(t, freeDNS.AddRecord("TXT", "", "\"TEST\"", false, ""))

	id, _ := freeDNS.FindRecord(SelectedDomain, "TXT", "\"TEST\"")
	require.NotEmpty(t, id)
	require.Nil(t, freeDNS.DeleteRecord(id))
	require.Nil(t, freeDNS.Logout())
	require.Equal(t, freeDNS.LoggedOut, true)
}
