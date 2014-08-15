lotf
====

Line Oriented Tail -f.
Go implementation of ``tail -f <file> | grep [-v] -f <filter file>''


lotf command
------------

args are multiple '\<filename\>:\<filter file name\>:\<number of last lines\>' in
which filename is required, others are optional. filter can be inverted by
putting '!' at head. so

    ./lotf 'testfile:!testfilter:10'

means

    tail -n 10 -f testfile | grep -v -f testfilter


lotf daemon
-----------

just outputing via tcp, udp. usage is:

    ./lotfd [-c <conf file>]
         [-o <logfile>] [-l <loglevel>] [-p <pidfile>]
         [-n <number of last lines>]

where conf file is json format:

    file: <target file>
    filter: <filter file>
    tcpaddr: tcp listening address
    udpaddr: udp sending address
    buflines: number of line in buffer

see lotfd/sample.json  


requires
--------

* go inotify (http://github.com/chamaken/inotify)
* logger (http://github.com/chamaken/logger)
