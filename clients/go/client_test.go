package golang

import "net/http"

func newTestClient() Client {
	return Client{
		Header: http.Header{
			"ContentType": []string{"application/json"},
		},
	}
}
