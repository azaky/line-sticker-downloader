# Line Sticker Downloader

This is a Line bot to download Line stickers as zip archive. It was initially 
intended so that we can use Line stickers in WhatsApp (that's why the archive
contains at most 20 stickers / folder), but the asset may be used elsewhere.

<img src="https://user-images.githubusercontent.com/5902356/69003142-aa177b80-092f-11ea-82ca-dd88f2ebe9a1.jpeg" width="256px">

## Getting Started

If you just want to use the bot, you may add it as friend [here](https://line.me/R/ti/p/%40160fzrqk).

But if you want to run it by yourself, you will first need a Line channel with
Messaging API capability (create it [here](https://developers.line.biz/). Also
make sure that Go v12+ is installed. Then:

```
go get github.com/azaky/line-sticker-downloader
CHANNEL_TOKEN="your channel token" \
CHANNEL_SECRET="your channel secret" \
HOST="..." \
line-sticker-downloader
```

`CHANNEL_TOKEN` and `CHANNEL_SECRET` can be found in Line developer console.
`HOST` is the the url of where your bot is hosted, without trailing slash (for
example, `HOST="https://linestickerdownloader.example.com").

After it is up and running, you will need to update the webhook URL in the
Line developer console, with url `$HOST/callback` (using the example above,
your webhook should be https://linestickerdownloader.example.com/callback).
