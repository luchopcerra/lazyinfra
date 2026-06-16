package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CredentialsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".aws", "credentials")
}

func ReadCredentials() (map[string]map[string]string, error) {
	path := CredentialsPath()
	if path == "" {
		return nil, fmt.Errorf("cannot determine home directory")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]map[string]string{}, nil
		}
		return nil, fmt.Errorf("read credentials: %w", err)
	}

	profiles := map[string]map[string]string{}
	var currentProfile string

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			currentProfile = line[1 : len(line)-1]
			if _, ok := profiles[currentProfile]; !ok {
				profiles[currentProfile] = map[string]string{}
			}
			continue
		}
		if currentProfile == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		profiles[currentProfile][strings.TrimSpace(key)] = strings.TrimSpace(value)
	}

	return profiles, nil
}

func WriteCredentials(profile string, creds *AWSCredentials) error {
	path := CredentialsPath()
	if path == "" {
		return fmt.Errorf("cannot determine home directory")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create .aws dir: %w", err)
	}

	existing, err := ReadCredentials()
	if err != nil {
		return fmt.Errorf("read existing credentials: %w", err)
	}

	var b strings.Builder
	for p, keys := range existing {
		if p == profile {
			continue
		}
		b.WriteString(fmt.Sprintf("[%s]\n", p))
		for k, v := range keys {
			b.WriteString(fmt.Sprintf("%s = %s\n", k, v))
		}
		b.WriteString("\n")
	}

	b.WriteString(fmt.Sprintf("[%s]\n", profile))
	b.WriteString(fmt.Sprintf("aws_access_key_id = %s\n", creds.AccessKeyID))
	b.WriteString(fmt.Sprintf("aws_secret_access_key = %s\n", creds.SecretAccessKey))
	if creds.SessionToken != "" {
		b.WriteString(fmt.Sprintf("aws_session_token = %s\n", creds.SessionToken))
	}
	if !creds.Expiration.IsZero() {
		b.WriteString(fmt.Sprintf("aws_expiration = %s\n", creds.Expiration.Format(time3339)))
	}

	if err := os.WriteFile(path, []byte(b.String()), 0600); err != nil {
		return fmt.Errorf("write credentials: %w", err)
	}

	return nil
}

const time3339 = "2006-01-02T15:04:05Z"
