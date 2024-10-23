# nsrip

![GitHub Release](https://img.shields.io/github/v/release/Macmod/nsrip) ![](https://img.shields.io/github/go-mod/go-version/Macmod/nsrip) ![](https://img.shields.io/github/languages/code-size/Macmod/nsrip) ![](https://img.shields.io/github/license/Macmod/nsrip) ![](https://img.shields.io/github/actions/workflow/status/Macmod/nsrip/release.yml) [![Go Report Card](https://goreportcard.com/badge/github.com/Macmod/nsrip)](https://goreportcard.com/report/github.com/Macmod/nsrip) ![GitHub Downloads](https://img.shields.io/github/downloads/Macmod/nsrip/total)[<img alt="Twitter Follow" src="https://img.shields.io/twitter/follow/MacmodSec?style=for-the-badge&logo=X&color=blue">](https://twitter.com/MacmodSec)

A fast and simple batch DNS resolver for A/AAAA/CNAME records from multiple nameservers bundled with lists of nameservers from popular DNS providers (AWS, Azure, GCP) designed to aid in finding origin DNS leaks for WAF bypasses and other research purposes.

# Usage

```
$ git clone https://github.com/Macmod/nsrip
$ cd nsrip
$ go build
$ ./nsrip -d <targetdomain>
```

# Flags

```
  -d, --domain string     Specify the target domain
  -l, --list string       Specify a file with a list of target domains
  -p, --provider string   Specify the nameserver list to use (aws, azure, gcp, cloud, or the path to a custom file) (default "cloud")
  -q, --quiet             Only output raw results
  -w, --workers int       Specify the number of workers (default 5)
```

# License

The MIT License (MIT)

Copyright (c) 2023 Artur Henrique Marzano Gonzaga

Permission is hereby granted, free of charge, to any person obtaining a copy of this software and associated documentation files (the "Software"), to deal in the Software without restriction, including without limitation the rights to use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
