package titanfall

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"

	"github.com/multiplay/go-svrquery/lib/svrquery/common"
	"github.com/multiplay/go-svrquery/lib/svrquery/protocol"
)

var (

	// minLength is the smallest packet we can expect.
	minLength         = 26
	tagSize           = 16
	packetSize        = 1200
	gcmAdditionalData = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
)

type queryer struct {
	c       protocol.Client
	version byte
}

func newQueryer(version byte) func(c protocol.Client) protocol.Queryer {
	return func(c protocol.Client) protocol.Queryer {
		return &queryer{
			c:       c,
			version: version,
		}
	}
}

// encrypt encrypts the byte buffer given to it.
func (q *queryer) encrypt(b []byte) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(q.c.Key())
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	c, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("read random nonce: %w", err)
	}

	// This will output in the form CipherTest | Tag and will need rearranging.
	cipherTextAndTag := gcm.Seal(nil, nonce, b, gcmAdditionalData)

	// Rearrange output to nonce | tag | ciphertext.
	newCipherText := nonce
	newCipherText = append(newCipherText, cipherTextAndTag[len(cipherTextAndTag)-tagSize:]...)
	newCipherText = append(newCipherText, cipherTextAndTag[:len(cipherTextAndTag)-tagSize]...)

	return newCipherText, nil
}

// decrypt decrypts the byte buffer given to it.
func (q *queryer) decrypt(b []byte) ([]byte, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(q.c.Key())
	if err != nil {
		return nil, fmt.Errorf("decode key: %w", err)
	}

	c, err := aes.NewCipher(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("new aes cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(c)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	if len(b) < gcm.NonceSize() {
		return nil, fmt.Errorf("incoming bytes smaller than %d", gcm.NonceSize())
	}

	nonce, tag, b := b[:gcm.NonceSize()], b[gcm.NonceSize():gcm.NonceSize()+tagSize], b[gcm.NonceSize()+tagSize:]
	b = append(b, tag...)
	plaintext, err := gcm.Open(nil, nonce, b, gcmAdditionalData)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// Query implements protocol.Queryer.
func (q *queryer) Query() (resp protocol.Responser, err error) {
	b := make([]byte, packetSize)
	copy(b, q.serverInfoPkt())

	// For older query versions we use a keyed magic section to auth. For newer versions we use encryption
	if key := q.c.Key(); key != "" {
		if q.version < 5 {
			// If keyed data asked for bump version sent to supported version level.
			b[5] = ServerInfoVersionKeyed
		}
		copy(b[6:], key)
	}

	if q.version >= 8 && q.c.Key() != "" {
		b, err = q.encrypt(b)
		if err != nil {
			return nil, err
		}
	}

	if _, err := q.c.Write(b); err != nil {
		return nil, fmt.Errorf("query write: %w", err)
	}

	n, err := q.c.Read(b)
	if err != nil {
		return nil, fmt.Errorf("query read: %w", err)
	} else if n < minLength {
		return nil, fmt.Errorf("packet too short (len: %d)", n)
	}

	if q.version >= 8 && q.c.Key() != "" {
		b, err = q.decrypt(b[:n])
		if err != nil {
			return nil, err
		}
	}

	r := common.NewBinaryReader(b, binary.LittleEndian)
	i := &Info{}

	// Header.
	if err = r.Read(&i.Header); err != nil {
		return nil, err
	} else if i.Command != ServerInfoResponse {
		return nil, fmt.Errorf("unexpected cmd %x", i.Command)
	}

	if i.Version > 1 {
		// InstanceInfo.
		if err = q.instanceInfo(r, i); err != nil {
			return nil, err
		}
	}

	// BasicInfo.
	if err = q.basicInfo(r, i); err != nil {
		return nil, err
	}

	if i.Version >= 9 {
		// PerformanceInfoV9.
		if err = r.Read(&i.PerformanceInfoV9); err != nil {
			return nil, err
		}
		i.PerformanceInfo = i.PerformanceInfoV9.PerformanceInfo
	} else if i.Version > 4 {
		// PerformanceInfo.
		if err = r.Read(&i.PerformanceInfo); err != nil {
			return nil, err
		}
	}

	if i.Version > 2 {
		if i.Version >= 10 {
			if err = r.Read(&i.MatchStateV10); err != nil {
				return nil, err
			}
			i.MatchStateV9 = i.MatchStateV10.MatchStateV9
			i.MatchStateV6 = i.MatchStateV10.toV6()
		} else if i.Version >= 9 {
			if err = r.Read(&i.MatchStateV9); err != nil {
				return nil, err
			}
			i.MatchStateV6 = i.MatchStateV9.toV6()
		} else if i.Version > 5 {
			// MatchStateV6 and Teams.
			if err = r.Read(&i.MatchStateV6); err != nil {
				return nil, err
			}
		} else if err = r.Read(&i.MatchStateV6.MatchStateV2); err != nil {
			return nil, err
		}

		if err = q.teams(r, i); err != nil {
			return nil, err
		}
	}

	// Clients
	if err = q.clients(r, i); err != nil {
		return nil, err
	}

	return i, nil
}

// instanceInfo decodes the instance information from a response.
func (q *queryer) instanceInfo(r *common.BinaryReader, i *Info) (err error) {
	if i.Version > 7 {
		if err = r.Read(&i.InstanceInfoV8); err != nil {
			return err
		}
		i.InstanceInfo = i.InstanceInfoV8.toV1()
	} else {
		if err = r.Read(&i.InstanceInfo); err != nil {
			return err
		}
	}
	if i.BuildName, err = r.ReadString(); err != nil {
		return err
	}
	if i.Datacenter, err = r.ReadString(); err != nil {
		return err
	}
	i.GameMode, err = r.ReadString()
	return err
}

// basicInfo decodes basic info from a response.
func (q *queryer) basicInfo(r *common.BinaryReader, i *Info) (err error) {
	if err = r.Read(&i.BasicInfo.Port); err != nil {
		return err
	} else if i.BasicInfo.Platform, err = r.ReadString(); err != nil {
		return err
	} else if i.BasicInfo.PlaylistVersion, err = r.ReadString(); err != nil {
		return err
	} else if err = r.Read(&i.BasicInfo.PlaylistNum); err != nil {
		return err
	} else if i.BasicInfo.PlaylistName, err = r.ReadString(); err != nil {
		return err
	}

	if i.Version > 6 {
		var platformNum byte

		if err = r.Read(&platformNum); err != nil {
			return err
		}
		i.BasicInfo.PlatformPlayers = make(map[string]byte, platformNum)

		for j := 0; j < int(platformNum); j++ {
			platformName, err := r.ReadString()
			if err != nil {
				return err
			}
			var platformPlayers byte
			if err = r.Read(&platformPlayers); err != nil {
				return err
			}
			i.BasicInfo.PlatformPlayers[platformName] = platformPlayers
		}
	}

	if err = r.Read(&i.BasicInfo.NumClients); err != nil {
		return err
	}

	if i.Version >= 9 {
		if err = r.Read(&i.BasicInfo.NumBotClients); err != nil {
			return err
		}
	}

	if err = r.Read(&i.BasicInfo.MaxClients); err != nil {
		return err
	}

	if i.Version >= 9 {
		if err = r.Read(&i.BasicInfo.TotalClientsConnectedEver); err != nil {
			return err
		}
	}

	i.BasicInfo.Map, err = r.ReadString()
	return err
}

// teams decodes teams from a response.
func (q *queryer) teams(r *common.BinaryReader, i *Info) (err error) {
	var id byte
	if err = r.Read(&id); err != nil {
		return err
	} else if id != 255 {
		i.Teams = make([]Team, 0, 2)
		for id != 255 {
			t := Team{ID: id}
			if err = r.Read(&t.Score); err != nil {
				return err
			}
			i.Teams = append(i.Teams, t)

			if err = r.Read(&id); err != nil {
				return err
			}
		}
	}
	return nil
}

// clients decodes clients from a response.
func (q *queryer) clients(r *common.BinaryReader, i *Info) (err error) {
	var id uint64
	if err = r.Read(&id); err != nil {
		return err
	}

	for id > 0 {
		c := Client{ID: id}
		c.Name, err = r.ReadString()
		if err != nil {
			return err
		} else if err = r.Read(&c.TeamID); err != nil {
			return err
		}

		if i.Version > 3 {
			c.Address, err = r.ReadString()
			if err != nil {
				return err
			} else if err = r.Read(&c.Ping); err != nil {
				return err
			} else if err = r.Read(&c.PacketsReceived); err != nil {
				return err
			} else if err = r.Read(&c.PacketsDropped); err != nil {
				return err
			}
		}

		if i.Version > 2 {
			if err = r.Read(&c.Score); err != nil {
				return err
			} else if err = r.Read(&c.Kills); err != nil {
				return err
			} else if err = r.Read(&c.Deaths); err != nil {
				return err
			}
		}
		i.Clients = append(i.Clients, c)

		if err = r.Read(&id); err != nil {
			return err
		}
	}

	return nil
}

// serverInfoPkt returns a byte array of info request packet data.
func (q *queryer) serverInfoPkt() []byte {
	return []byte{0xFF, 0xFF, 0xFF, 0xFF, ServerInfoRequest, q.version}
}
