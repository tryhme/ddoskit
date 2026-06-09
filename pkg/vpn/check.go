package vpn

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

type Result struct {
	Active bool
	IP     string
	Org    string
	ASN    string
}

var client = &http.Client{Timeout: 6 * time.Second}

// Check detecta ProtonVPN usando múltiples métodos en cascada
func Check() Result {
	// Método 1: API propia de ProtonVPN
	if ok, ip := checkProtonAPI(); ok {
		return Result{Active: true, IP: ip, Org: "ProtonVPN", ASN: "ProtonVPN"}
	}

	// Método 2: icanhazip (plain text, muy confiable)
	ip := getIP()
	if ip == "" {
		return Result{Active: false, IP: "sin conexión"}
	}

	// Método 3: ipinfo.io para org/ASN
	org, asn := getOrgASN(ip)

	active := isVPN(org, asn)
	return Result{Active: active, IP: ip, Org: org, ASN: asn}
}

// checkProtonAPI usa el endpoint oficial de ProtonVPN
func checkProtonAPI() (bool, string) {
	resp, err := client.Get("https://api.protonvpn.ch/vpn/location")
	if err != nil {
		return false, ""
	}
	defer resp.Body.Close()

	var data struct {
		IP      string `json:"IP"`
		Country string `json:"Country"`
		ISP     string `json:"ISP"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return false, ""
	}
	// Si respondió la API de ProtonVPN con IP válida = estamos en ProtonVPN
	if data.IP != "" {
		return true, data.IP
	}
	return false, ""
}

// getIP obtiene la IP pública via múltiples fuentes
func getIP() string {
	apis := []string{
		"https://icanhazip.com",
		"https://api64.ipify.org",
		"https://checkip.amazonaws.com",
		"https://ipecho.net/plain",
	}
	for _, api := range apis {
		resp, err := client.Get(api)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		ip := strings.TrimSpace(string(body))
		if ip != "" && !strings.Contains(ip, "<") {
			return ip
		}
	}
	return ""
}

// getOrgASN obtiene org y ASN de una IP
func getOrgASN(ip string) (string, string) {
	apis := []struct {
		url    string
		orgKey string
		asnKey string
	}{
		{"https://ipapi.co/" + ip + "/json/", "org", "asn"},
		{"https://ipinfo.io/" + ip + "/json", "org", ""},
	}

	for _, api := range apis {
		resp, err := client.Get(api.url)
		if err != nil {
			continue
		}
		var data map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&data)
		resp.Body.Close()

		org := ""
		asn := ""
		if v, ok := data[api.orgKey].(string); ok {
			org = v
		}
		if api.asnKey != "" {
			if v, ok := data[api.asnKey].(string); ok {
				asn = v
			}
		}
		if org != "" {
			return org, asn
		}
	}
	return "", ""
}

var knownOrgs = []string{
	"proton", "m247", "datapacket", "aeza", "vpn",
	"mullvad", "expressvpn", "nordvpn", "perfect privacy",
	"surfshark", "cyberghost", "privateinternetaccess",
}

var knownASNs = []string{
	"AS212238", "AS9009", "AS196714", "AS208843",
	"AS23576", "AS60068", "AS51167", "AS209854",
}

func isVPN(org, asn string) bool {
	orgLow := strings.ToLower(org)
	for _, k := range knownOrgs {
		if strings.Contains(orgLow, k) {
			return true
		}
	}
	for _, a := range knownASNs {
		if strings.EqualFold(asn, a) {
			return true
		}
	}
	return false
}
