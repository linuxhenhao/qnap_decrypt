# Qnap HBS3 backup file decrypt

this is a tool for decrypting client side encrypted file.

for a single file backuped by HBS3 backup job, run file, the output is:
```text
xxx: openssl enc'd data with salted password
```
So, It's a openssl encrpyted file with salted password;

There is a very good project that support many version of HBS encryptions: [hbs_decipher](https://github.com/Mikiya83/hbs_decipher)
But it was implemented in java and it takes some effort to run such a program, especially for none programmers.

Therefore, here is a version implemented in go.

# install and usage

```bash
go install github.com/linuxhenhao/qnap_decrypt
qnap_decrypt -i input_path -o output_path -k password
```