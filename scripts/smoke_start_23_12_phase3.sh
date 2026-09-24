#!/bin/sh
set -eu

BASE="${1:-http://127.0.0.1:18082}"
export BASE
python3 - <<'PY'
import hashlib,hmac,json,os,time,urllib.request,urllib.error

BASE=os.environ["BASE"].rstrip("/")
TOKEN="local-development-internal-token-123456789"
VERSION="0.8.32-start-23.11.7"
KEYS={
    "ci-producer":"ci-producer-automation-secret-123456789",
    "ci-consumer":"ci-consumer-automation-secret-123456789",
}

def sign(service,method,path,body=b""):
    ts=str(int(time.time()))
    digest=hashlib.sha256(body).hexdigest()
    canonical="\n".join([ts,service,method.upper(),path,digest]).encode()
    sig="sha256="+hmac.new(KEYS[service].encode(),canonical,hashlib.sha256).hexdigest()
    return {
        "X-Himate-Internal-Token":TOKEN,
        "X-Himate-Expected-Version":VERSION,
        "X-Himate-Service-ID":service,
        "X-Himate-Service-Timestamp":ts,
        "X-Himate-Service-Signature":sig,
        "Content-Type":"application/json",
    }

def request(service,method,path,payload=None,expected=(200,201)):
    body=b"" if payload is None else json.dumps(payload,separators=(",",":"),sort_keys=True).encode()
    req=urllib.request.Request(BASE+path,data=body if method!="GET" else None,method=method,headers=sign(service,method,path,body))
    try:
        with urllib.request.urlopen(req,timeout=10) as r:
            data=json.loads(r.read() or b"{}")
            if r.status not in expected: raise AssertionError((r.status,data))
            return r.status,data
    except urllib.error.HTTPError as e:
        data=json.loads(e.read() or b"{}")
        if e.code not in expected: raise AssertionError((e.code,data))
        return e.code,data

# Register a consumer with one-attempt dead-letter behavior.
_,sub=request("ci-consumer","POST","/internal/v1/automation/subscriptions",{
    "event_types":["workflow.qc.passed.v1"],
    "max_attempts":1,
})
assert sub["consumer_service"]=="ci-consumer"

now=time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime())
event={
    "event_key":"phase3-smoke-qc-1",
    "event_type":"workflow.qc.passed.v1",
    "event_version":1,
    "partner_id":"ptr_phase3",
    "module_key":"workshop_workflow",
    "correlation_id":"corr-phase3-1",
    "subject_type":"work_order",
    "subject_id":"wo-1",
    "payload":{"result":"PASS"},
    "occurred_at":now,
}
status,created=request("ci-producer","POST","/internal/v1/automation/events",event)
assert status==201 and created["duplicate"] is False
status,dup=request("ci-producer","POST","/internal/v1/automation/events",event)
assert status==200 and dup["duplicate"] is True

changed=dict(event);changed["payload"]={"result":"CHANGED"}
status,conflict=request("ci-producer","POST","/internal/v1/automation/events",changed,expected=(409,))
assert status==409

_,claim=request("ci-consumer","POST","/internal/v1/automation/deliveries/claim",{"limit":10})
assert claim["count"]==1,claim
item=claim["items"][0]
ev=item["event"]
assert ev["partner_id"]=="ptr_phase3"
assert ev["module_key"]=="workshop_workflow"
assert ev["correlation_id"]=="corr-phase3-1"

delivery_id=item["delivery_id"]
_,failed=request("ci-consumer","POST",f"/internal/v1/automation/deliveries/{delivery_id}/fail",{"error":"phase3 smoke forced failure","retry_after_seconds":5})
assert failed["status"]=="DEAD_LETTER"

_,dead=request("ci-consumer","GET","/internal/v1/automation/dead-letters")
assert any(x["delivery_id"]==delivery_id for x in dead["items"])

# A future event must persist now but must not be claimable before available_at.
future=time.strftime("%Y-%m-%dT%H:%M:%SZ",time.gmtime(time.time()+120))
future_event=dict(event)
future_event["event_key"]="phase3-smoke-qc-future"
future_event["subject_id"]="wo-future"
future_event["available_at"]=future
request("ci-producer","POST","/internal/v1/automation/events",future_event)
_,future_claim=request("ci-consumer","POST","/internal/v1/automation/deliveries/claim",{"limit":10})
assert future_claim["count"]==0,future_claim

# Service identity must reject tampered signatures.
path="/internal/v1/automation/deliveries/claim"
body=b'{"limit":1}'
headers=sign("ci-consumer","POST",path,body)
headers["X-Himate-Service-Signature"]="sha256="+"0"*64
req=urllib.request.Request(BASE+path,data=body,method="POST",headers=headers)
try:
    urllib.request.urlopen(req,timeout=10)
    raise AssertionError("tampered service signature was accepted")
except urllib.error.HTTPError as e:
    assert e.code==403,e.code

print("START-23.12 Phase 3 automation backbone smoke: PASS")
PY
