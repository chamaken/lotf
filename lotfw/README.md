lotfw
=====

lotf on web.

``tail --follow=name <ファイル名> -n <行数> | grep [-v] -f <フィルターファイル名>'' を Web で閲覧します。
inotify を使っているため linux 上でしか動きません。


インストール
------------

Go (http://golang.org) がインストール済みで環境変数も設定されているとします。

    # wget https://storage.googleapis.com/golang/go1.3.1.linux-amd64.tar.gz
    # tar xzf go1.3.1.linux-amd64.tar.gz -C /usr/local
    # ln -sf /usr/local/go1.3.1 /usr/local/go

とした後に一般ユーザーで

    $ export GOHOME=/usr/local/go
    $ export PATH=$PATH:$GOHOME/bin
    $ export GOPATH=~/gopath
    $ mkdir $GOPATH

を例とします。lotfw のインストールは

    $ go get github.com/chamaken/lotf
    $ cd $GOPATH/src/github.com/chamaken/lotf/lotfw
    $ go build

です。この後の説明ではカレントディレクトリを
$GOPATH/src/github.com/chamaken/lotf/lotfw としています。


使用方法
--------

サンプルでの使い方だけ紹介します。ターミナル二つと Web ブラウザを起動してください。
カレントディレクトリを $GOPATH/src/github.com/chamaken/lotf/lotfw とした後に、ター
ミナル 1 で

    $ ./lotfw -c etc/sample.json

を実行して Web ブラウザで

    http://127.0.0.1:8088/lotf/sample

をアクセスします。後にブラウザ眺めながらターミナル 2 で

    $ echo "line 11" >> etc/sample_file
    $ echo >> etc/sample_file
    $ echo "line 20" >> etc/sample_file
    $ echo "line 12" >> etc/sample_file
    $ rm etc/sample_file
    $ echo "line 100" >> etc/sample_file

など実行してみてください。


その他、所感
------------

ログファイルを眺めるにあたって、内部で使っている同じような Web アプリケーションが
評判良く、他でも使いたいとの話がありましたので go の http を使って実装してみまし
た。html / js / css は jQuery と bootstrap を使っているものの、おなぐさみ程度で
すので適宜改良してください。
