package api

// ListServers returns all servers
func (c *Client) ListServers() ([]Server, error) {
	var servers []Server
	err := c.Get("/servers", &servers)
	return servers, err
}

// GetServer returns a server by UUID
func (c *Client) GetServer(uuid string) (*Server, error) {
	var server Server
	err := c.Get("/servers/"+uuid, &server)
	return &server, err
}
