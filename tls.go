package main

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type manager struct {
	path         string
	client       *lego.Client
	user         *user
	domains      []string
	certificates map[string]*tls.Certificate
	lock         sync.RWMutex
	jobLock      sync.Mutex
	jobRun       bool
}

func NewCertConfig(path string, defaultEmail string, domains []string, provider challenge.Provider) (*tls.Config, error) {
	// Create a user and initialize lego
	myUser, err := newUser(defaultEmail, path)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	config := lego.NewConfig(myUser)
	config.Certificate.KeyType = certcrypto.RSA2048

	client, err := lego.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create lego client: %w", err)
	}

	err = client.Challenge.SetHTTP01Provider(provider)
	if err != nil {
		return nil, fmt.Errorf("failed to set HTTP01 provider: %w", err)
	}

	err = myUser.register(client)
	if err != nil {
		return nil, fmt.Errorf("failed to register user: %w", err)
	}

	mgr := &manager{
		path:         path,
		client:       client,
		user:         myUser,
		domains:      domains,
		certificates: make(map[string]*tls.Certificate),
		jobRun:       false,
	}

	return &tls.Config{
		GetCertificate: mgr.getCertificate,
	}, nil
}

func (c *manager) getCertificate(info *tls.ClientHelloInfo) (*tls.Certificate, error) {
	c.lock.RLock()
	defer c.lock.RUnlock()

	cert, ok := c.certificates[info.ServerName]
	expiry := time.Now().Add(time.Hour * 24 * 31)
	needsUpdate := !ok || cert.Leaf.NotAfter.Before(expiry)

	// we will only launch a single instance of the update function until it succeeds
	if needsUpdate && !c.jobRun {
		c.jobLock.Lock()
		start := !c.jobRun
		c.jobRun = true
		c.jobLock.Unlock()

		if start {
			go func() {
				err := c.updateCert(info.ServerName, ok)
				if err != nil {
					log.Err(err).Msg("failed to update certificate")
				}
			}()
		}
	}

	if !ok {
		return nil, fmt.Errorf("certificate not found for %s", info.ServerName)
	}

	return cert, nil
}

type Certificate struct {
	Certificate string `json:"certificate"`
	PrivateKey  string `json:"privateKey"`
}

func (c *manager) updateCert(domain string, forceUpdate bool) error {
	defer func() {
		c.jobLock.Lock()
		c.jobRun = false
		c.jobLock.Unlock()
	}()

	// Check if domain is allowed
	allowed := false
	for _, d := range c.domains {
		if d == domain {
			allowed = true
			break
		}
	}

	if !allowed {
		return fmt.Errorf("domain %s not allowed", domain)
	}

	filePath := filepath.Join(c.path, domain+".json")

	cert := new(Certificate)
	err := read(filePath, cert)
	if err == nil && !forceUpdate {
		certB, err := base64.StdEncoding.DecodeString(cert.Certificate)
		if err != nil {
			return fmt.Errorf("failed to decode certificate: %w", err)
		}

		keyB, err := base64.StdEncoding.DecodeString(cert.PrivateKey)
		if err != nil {
			return fmt.Errorf("failed to decode key: %w", err)
		}

		return c.setCertificateFromPem(domain, certB, keyB)
	}

	certificates, err := c.client.Certificate.Obtain(certificate.ObtainRequest{
		Domains: []string{domain},
		Bundle:  true,
	})

	if err != nil {
		return fmt.Errorf("failed to obtain certificates: %w", err)
	}

	cert.Certificate = base64.StdEncoding.EncodeToString(certificates.Certificate)
	cert.PrivateKey = base64.StdEncoding.EncodeToString(certificates.PrivateKey)

	err = write(filePath, cert)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return c.setCertificateFromPem(domain, certificates.Certificate, certificates.PrivateKey)
}

func (c *manager) setCertificateFromPem(domain string, certPem, keyPem []byte) error {
	cert, err := tls.X509KeyPair(certPem, keyPem)
	if err != nil {
		return fmt.Errorf("failed to parse X509 key pair: %w", err)
	}

	x509Cert, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	cert.Leaf = x509Cert

	c.lock.Lock()
	c.certificates[domain] = &cert
	c.lock.Unlock()

	return nil
}

type user struct {
	email        string
	key          *ecdsa.PrivateKey
	registration *registration.Resource
}

type serializableUser struct {
	Email      string `json:"email"`
	PrivateKey string `json:"privateKey"`
}

func newUser(email string, path string) (*user, error) {
	filePath := filepath.Join(path, "user.json")
	u := new(user)

	su := new(serializableUser)
	err := read(filePath, su)
	if err == nil {
		u.email = su.Email

		log.Info().Str("email", u.email).Str("key", su.PrivateKey).Msg("loaded user from file")

		privKey, err := base64.StdEncoding.DecodeString(su.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decode private key: %w", err)
		}

		key, err := x509.ParseECPrivateKey(privKey)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}

		u.key = key
	} else {
		log.Err(err).Msg("no user found - creating new one")
	}

	if u.email == "" {
		u.email = email
	}

	if u.key == nil {
		key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, fmt.Errorf("failed to generate private key: %w", err)
		}

		u.key = key
	}

	// Finally write it back to the file
	su.Email = u.email
	privKey, err := x509.MarshalECPrivateKey(u.key)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	su.PrivateKey = base64.StdEncoding.EncodeToString(privKey)

	err = write(filePath, su)

	return u, err
}

func (u *user) register(client *lego.Client) error {
	// New users will need to register - but let's try to get the user first
	reg, err := client.Registration.ResolveAccountByKey()
	if err != nil {
		log.Err(err).Msg("failed to resolve account by key - will try to register")
		reg, err = client.Registration.Register(registration.RegisterOptions{TermsOfServiceAgreed: true})
		if err != nil {
			return fmt.Errorf("failed to register: %w", err)
		}
	}

	u.registration = reg

	return err
}

func (u *user) GetEmail() string {
	return u.email
}

func (u *user) GetRegistration() *registration.Resource {
	return u.registration
}

func (u *user) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// Copy reader buffer to file
func write(filePath string, obj interface{}) error {
	// Generating unique temporary file name
	tempFilePath := fmt.Sprintf("%s_%s", filePath, uuid.New().String())

	file, err := os.OpenFile(tempFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Err(err).Msg("failed to close file")
		}
	}(file)

	// Write to file
	err = json.NewEncoder(file).Encode(obj)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	// Replace the original file with the temporary one
	if err := os.Rename(tempFilePath, filePath); err != nil {
		fmt.Println("Error while renaming temporary file", err)
		return err
	}

	return nil
}

func read(filePath string, obj interface{}) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}

	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			log.Err(err).Msg("failed to close file")
		}
	}(file)

	err = json.NewDecoder(file).Decode(obj)
	if err != nil {
		return fmt.Errorf("failed to decode file: %w", err)
	}

	return nil
}

type HttpProvider struct {
	keys map[string]string
	lock sync.RWMutex
}

func NewHttpProvider() *HttpProvider {
	return &HttpProvider{
		keys: make(map[string]string),
	}
}

func (p *HttpProvider) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	p.lock.RLock()
	defer p.lock.RUnlock()

	if !strings.HasPrefix(req.URL.Path, "/.well-known/acme-challenge/") {
		http.Redirect(rw, req, "https://"+req.Host+req.RequestURI, http.StatusMovedPermanently)
		return
	}

	domain := req.Host
	token := strings.TrimPrefix(req.URL.Path, "/.well-known/acme-challenge/")

	keyAuth, ok := p.keys[domain+"-"+token]
	if !ok {
		http.Error(rw, "key not found", http.StatusNotFound)
		return
	}

	_, err := rw.Write([]byte(keyAuth))
	if err != nil {
		http.Error(rw, "failed to write keyAuth", http.StatusInternalServerError)
		return
	}
}

func (p *HttpProvider) Present(domain, token, keyAuth string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.keys[domain+"-"+token] = keyAuth
	return nil
}

func (p *HttpProvider) CleanUp(domain, token, _ string) error {
	p.lock.Lock()
	defer p.lock.Unlock()

	delete(p.keys, domain+"-"+token)
	return nil
}
