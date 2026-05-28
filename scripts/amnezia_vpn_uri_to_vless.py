#!/usr/bin/env python3
"""
Decode Amnezia export link vpn://... (4-byte header + zlib + JSON) and print
the first supported proxy URI found in containers[].xray.last_config.

Supported generated schemes:
- vless://
- trojan://
- ss://
- awg2:// (synthetic URI for podkop failover helper)

This keeps compatibility with podkop sing_box_cf_add_proxy_outbound() parser.
"""
from __future__ import annotations

import base64
import json
import sys
import zlib


def _b64url_decode(data: str) -> bytes:
    pad = "=" * ((4 - len(data) % 4) % 4)
    return base64.urlsafe_b64decode(data + pad)


def _load_amnezia_export(raw: bytes) -> dict:
    if len(raw) < 8:
        raise ValueError("vpn payload too short")
    return json.loads(zlib.decompress(raw[4:]))


def _pct_encode(text: str) -> str:
    safe = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~"
    out: list[str] = []
    for b in text.encode("utf-8"):
        ch = chr(b)
        if ch in safe:
            out.append(ch)
        else:
            out.append(f"%{b:02X}")
    return "".join(out)


def _urlencode_map(items: dict[str, str]) -> str:
    return "&".join(
        f"{_pct_encode(str(k))}={_pct_encode(str(v))}" for k, v in items.items()
    )


def _b64_no_pad(text: str) -> str:
    return base64.urlsafe_b64encode(text.encode("utf-8")).decode("ascii").rstrip("=")


def _xray_stream_to_query(ss: dict) -> dict[str, str]:
    q: dict[str, str] = {}
    net = ss.get("network") or "tcp"
    q["type"] = net
    sec = ss.get("security") or "none"
    q["security"] = sec
    if sec == "reality":
        rs = ss.get("realitySettings") or {}
        if rs.get("serverName"):
            q["sni"] = rs["serverName"]
        if rs.get("fingerprint"):
            q["fp"] = rs["fingerprint"]
        if rs.get("publicKey"):
            q["pbk"] = rs["publicKey"]
        if rs.get("shortId") is not None:
            q["sid"] = str(rs["shortId"])
    elif sec == "tls":
        ts = ss.get("tlsSettings") or {}
        if ts.get("serverName"):
            q["sni"] = ts["serverName"]
        if ts.get("fingerprint"):
            q["fp"] = ts["fingerprint"]
    if net == "tcp":
        tcp = ss.get("tcpSettings") or {}
        header = tcp.get("header") or {}
        q["headerType"] = header.get("type") or "none"
    return q


def _vless_outbound_to_uri(ob: dict) -> str:
    if ob.get("protocol") != "vless":
        raise ValueError("first outbound is not vless")
    vnext = ob["settings"]["vnext"][0]
    host = vnext["address"]
    port = int(vnext["port"])
    user = vnext["users"][0]
    uuid = user["id"]
    q: dict[str, str] = {}
    enc = user.get("encryption")
    if enc:
        q["encryption"] = enc
    if user.get("flow"):
        q["flow"] = user["flow"]
    ss = ob.get("streamSettings") or {}
    q.update(_xray_stream_to_query(ss))
    qs = _urlencode_map(q)
    return f"vless://{uuid}@{host}:{port}?{qs}"


def _trojan_outbound_to_uri(ob: dict) -> str:
    if ob.get("protocol") != "trojan":
        raise ValueError("outbound is not trojan")
    srv = (ob.get("settings") or {}).get("servers", [{}])[0]
    host = srv["address"]
    port = int(srv["port"])
    password = srv["password"]
    q = _xray_stream_to_query(ob.get("streamSettings") or {})
    qs = _urlencode_map(q)
    return f"trojan://{_pct_encode(password)}@{host}:{port}?{qs}"


def _shadowsocks_outbound_to_uri(ob: dict) -> str:
    if ob.get("protocol") != "shadowsocks":
        raise ValueError("outbound is not shadowsocks")
    srv = (ob.get("settings") or {}).get("servers", [{}])[0]
    host = srv["address"]
    port = int(srv["port"])
    method = srv["method"]
    password = srv["password"]
    userinfo = _b64_no_pad(f"{method}:{password}")
    return f"ss://{userinfo}@{host}:{port}"


def _xray_outbound_to_proxy_uri(ob: dict) -> str | None:
    proto = (ob.get("protocol") or "").lower()
    if proto == "vless":
        return _vless_outbound_to_uri(ob)
    if proto == "trojan":
        return _trojan_outbound_to_uri(ob)
    if proto == "shadowsocks":
        return _shadowsocks_outbound_to_uri(ob)
    return None


def _amnezia_awg2_to_uri(awg: dict) -> str:
    lc = awg.get("last_config")
    inner = json.loads(lc) if isinstance(lc, str) else (lc or {})
    host = inner.get("hostName") or awg.get("hostName") or ""
    port = inner.get("port") or awg.get("port") or ""
    q: dict[str, str] = {}

    # Interface
    if inner.get("client_ip"):
        q["address"] = f"{inner['client_ip']}/32"
    if inner.get("client_priv_key"):
        q["private_key"] = inner["client_priv_key"]
    if inner.get("mtu"):
        q["mtu"] = str(inner["mtu"])

    # Peer
    if inner.get("server_pub_key"):
        q["public_key"] = inner["server_pub_key"]
    if inner.get("psk_key"):
        q["preshared_key"] = inner["psk_key"]
    q["allowed_ips"] = "0.0.0.0/0,::/0"
    if inner.get("persistent_keep_alive"):
        q["persistent_keepalive"] = str(inner["persistent_keep_alive"])

    # AmneziaWG params
    for key in (
        "Jc",
        "Jmin",
        "Jmax",
        "S1",
        "S2",
        "S3",
        "S4",
        "H1",
        "H2",
        "H3",
        "H4",
        "I1",
        "I2",
        "I3",
        "I4",
        "I5",
    ):
        value = inner.get(key)
        if value is not None and str(value) != "":
            q[key.lower()] = str(value)

    qs = _urlencode_map(q)
    return f"awg2://{host}:{port}?{qs}"


def vpn_uri_to_proxy(uri: str) -> str:
    uri = uri.strip()
    if not uri.lower().startswith("vpn://"):
        raise ValueError("expected vpn:// URI")
    b64 = uri.split("://", 1)[1].strip()
    root = _load_amnezia_export(_b64url_decode(b64))
    containers = root.get("containers") or []
    protocols: list[str] = []
    containers_seen: list[str] = []
    for c in containers:
        container_name = str(c.get("container") or "")
        if container_name:
            containers_seen.append(container_name)
        if c.get("container") != "amnezia-xray":
            if c.get("container") == "amnezia-awg2":
                awg = c.get("awg") or {}
                return _amnezia_awg2_to_uri(awg)
            continue
        xray = c.get("xray") or {}
        lc = xray.get("last_config")
        if not lc:
            continue
        inner = json.loads(lc) if isinstance(lc, str) else lc
        for ob in inner.get("outbounds") or []:
            p = str(ob.get("protocol") or "")
            if p:
                protocols.append(p)
            uri_out = _xray_outbound_to_proxy_uri(ob)
            if uri_out:
                return uri_out
    raise ValueError(
        "no supported outbound in amnezia-xray last_config "
        f"(containers: {', '.join(containers_seen) if containers_seen else 'none'}, "
        f"protocols: {', '.join(protocols) if protocols else 'none'})"
    )


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: amnezia_vpn_uri_to_vless.py 'vpn://...'", file=sys.stderr)
        return 2
    try:
        print(vpn_uri_to_proxy(sys.argv[1]), end="")
    except Exception as e:
        print(f"amnezia_vpn_uri_to_vless: {e}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
