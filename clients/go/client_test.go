package golang

import "net/http"

func newTestClient() Client {
	return Client{
		header: http.Header{
			"ContentType": []string{"application/json"},
		},
	}
}
