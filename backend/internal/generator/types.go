package generator

type RealityOpts struct {
	PublicKey string `yaml:"public-key,omitempty"`
	ShortID   string `yaml:"short-id,omitempty"`
}

type XHTTPOpts struct {
	Path string `yaml:"path,omitempty"`
	Host string `yaml:"host,omitempty"`
	Mode string `yaml:"mode,omitempty"`
}

type Proxy struct {
	Name              string      `yaml:"name"`
	Type              string      `yaml:"type"`
	Server            string      `yaml:"server"`
	Port              int         `yaml:"port"`
	UUID              string      `yaml:"uuid,omitempty"`
	Flow              string      `yaml:"flow,omitempty"`
	Network           string      `yaml:"network,omitempty"`
	TLS               bool        `yaml:"tls,omitempty"`
	UDP               bool        `yaml:"udp,omitempty"`
	ALPN              []string    `yaml:"alpn,omitempty"`
	ServerName        string      `yaml:"servername,omitempty"`
	ClientFingerprint string      `yaml:"client-fingerprint,omitempty"`
	Encryption        *string     `yaml:"encryption,omitempty"`
	RealityOpts       RealityOpts `yaml:"reality-opts,omitempty"`
	XHTTPOpts         XHTTPOpts   `yaml:"xhttp-opts,omitempty"`
}
