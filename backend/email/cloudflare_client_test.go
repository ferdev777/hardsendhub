package email

import "testing"

func TestCloudflareClientPrefersDirectAPIWhenCredentialsPresent(t *testing.T) {
	tests := []struct {
		name       string
		accountID  string
		apiToken   string
		workerURL  string
		wantDirect bool
	}{
		{
			name:       "direct API preferred even when CF_WORKER_URL is set for R2/D1 sync",
			accountID:  "acct-123",
			apiToken:   "tok-abc",
			workerURL:  "https://app.facturasvideodigital.com",
			wantDirect: true,
		},
		{
			name:       "direct API used when workerURL is empty",
			accountID:  "acct-123",
			apiToken:   "tok-abc",
			workerURL:  "",
			wantDirect: true,
		},
		{
			name:       "falls back to worker proxy when API credentials missing",
			accountID:  "",
			apiToken:   "",
			workerURL:  "https://email-proxy.example.com",
			wantDirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := NewCloudflareClient(tt.accountID, tt.apiToken, tt.workerURL, "w-tok", "facturas@facturasvideodigital.com", 15)
			if err != nil {
				t.Fatalf("NewCloudflareClient() error = %v", err)
			}
			if got := client.isDirectAPI(); got != tt.wantDirect {
				t.Fatalf("isDirectAPI() = %v, want %v", got, tt.wantDirect)
			}
		})
	}
}
