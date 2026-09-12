package observability

import "testing"

func TestParseServerEndpointUsesLogicalGRPCTarget(t *testing.T) {
	tests := []struct {
		name        string
		target      string
		wantAddress string
		wantPort    int
	}{
		{name: "empty", target: "", wantAddress: "", wantPort: 0},
		{name: "host and port", target: "grpc.io:50051", wantAddress: "grpc.io", wantPort: 50051},
		{name: "IPv6 and port", target: "[2001:db8::1]:50051", wantAddress: "2001:db8::1", wantPort: 50051},
		{name: "DNS target", target: "dns:///grpc.io:50051", wantAddress: "grpc.io", wantPort: 50051},
		{name: "DNS target with resolver", target: "dns://1.2.3.4/grpc.io:50051", wantAddress: "grpc.io", wantPort: 50051},
		{name: "Unix socket", target: "unix:///run/example.sock", wantAddress: "/run/example.sock", wantPort: 0},
		{name: "unknown resolver", target: "zk://zookeeper:2181/my-server", wantAddress: "zk://zookeeper:2181/my-server", wantPort: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			address, port := parseServerEndpoint(tt.target)
			if address != tt.wantAddress || port != tt.wantPort {
				t.Fatalf("parseServerEndpoint(%q) = (%q, %d), want (%q, %d)", tt.target, address, port, tt.wantAddress, tt.wantPort)
			}
		})
	}
}
