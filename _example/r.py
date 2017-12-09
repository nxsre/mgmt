import redis
import re
import json

class Monitor():
    def __init__(self, connection_pool):
        self.connection_pool = connection_pool
        self.connection = None

    def __del__(self):
        try:
            self.reset()
        except:
            pass

    def reset(self):
        if self.connection:
            self.connection_pool.release(self.connection)
            self.connection = None

    def monitor(self):
        if self.connection is None:
            self.connection = self.connection_pool.get_connection(
                'monitor', None)
        self.connection.send_command("monitor")
        return self.listen()

    def parse_response(self):
        return self.connection.read_response()

    def listen(self):
        while True:
            yield self.parse_response()

if  __name__ == '__main__':
    # pip install git+https://github.com/dmeranda/demjson.git
    import demjson
    pool = redis.ConnectionPool(host='10.10.89.127',port=7777, db=0)
    monitor = Monitor(pool)
    commands = monitor.monitor()
    for c in commands :
        # print(c)
        p = re.compile(r'^(b)?[0-9]{10}\.[0-9]+\s\[[0-9]+\s[^\]]+\]\s([^\s]+)\s([^\s]+)\s(.*)')
        po = p.findall(str(c))
        # print(str(c))
        if len(po) >0:
            if "RPUSH" in str(po[0][1]):
                if len(po[0])>=4:
                    print(po[0][2])
                    jsonstr = po[0][3]
                    print(jsonstr)
                    
                    jsonstr = jsonstr.decode('utf-8','ignore')
                    # print(demjson.decode(jsonstr,encoding='utf8').decode('utf8','ignore'))
                    # print(json.dumps(json.loads(demjson.decode(jsonstr.replace("\\\\","\\"))), indent=4, sort_keys=False, ensure_ascii=False))
                    # print(json.dumps(json.loads(demjson.decode(jsonstr,encoding='utf8')), indent=4, sort_keys=False, ensure_ascii=False))
            