package encoder

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

type Encryptor struct {
	recipient *age.ScryptRecipient
	identity  *age.ScryptIdentity
}

// NewEncryptor cria encoder com senha
func NewEncryptor(password string) (*Encryptor, error) {
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create recipient: %w", err)
	}

	identity, err := age.NewScryptIdentity(password)
	if err != nil {
		return nil, fmt.Errorf("failed to create identity: %w", err)
	}

	return &Encryptor{
		recipient: recipient,
		identity:  identity,
	}, nil
}

// NewWriter retorna writer que criptografa em streaming
func (e *Encryptor) NewWriter(output io.Writer) (io.WriteCloser, error) {
	return age.Encrypt(output, e.recipient)
}

// ✅ DecryptReader retorna reader que descriptografa em streaming
func (e *Encryptor) DecryptReader(input io.Reader) (io.Reader, error) {
	return age.Decrypt(input, e.identity)
}

// Decrypt descriptografa arquivo completo
func (e *Encryptor) Decrypt(inputPath, outputPath string) error {
	in, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open encrypted file: %w", err)
	}
	defer func() {
		_ = in.Close()
	}()

	reader, err := age.Decrypt(in, e.identity)
	if err != nil {
		return fmt.Errorf("failed to decrypt (wrong password?): %w", err)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output: %w", err)
	}
	defer func() {
		_ = out.Close()
	}()

	if _, err := io.Copy(out, reader); err != nil {
		return fmt.Errorf("failed to write decrypted data: %w", err)
	}

	return nil
}
