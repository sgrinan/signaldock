package endpoint

import "uuid"

func testEndpoint(rawURL string) Endpoint {
	return Endpoint{
		ID:  uuid.NewV7(),
		URL: rawURL,
	}
}
