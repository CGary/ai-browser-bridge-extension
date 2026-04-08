# Chromium Extension Identity

- Change: `t7-inicializacion-extension`
- Native host name: `aibbe`
- Extension ID: `bedlojjaiogmaefoadfpdecgajipcpgj`
- Allowed origin for t8: `chrome-extension://bedlojjaiogmaefoadfpdecgajipcpgj/`

## Key generation

Generate the private key locally and keep it out of version control:

```bash
openssl genrsa 2048 | openssl pkcs8 -topk8 -nocrypt -out key.pem
openssl rsa -in key.pem -pubout -outform DER | base64 -w 0
openssl rsa -in key.pem -pubout -outform DER | sha256sum | head -c 32 | tr '0-9a-f' 'a-p'
```

The current public key is embedded in `extension/manifest.json`. The matching private key is the ignored local file `key.pem`.
