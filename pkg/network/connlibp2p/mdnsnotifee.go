package connlibp2p

import (
	"context"
	"log/slog"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

type mdnsNotifee struct {
	h host.Host
}

func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	slog.Info("mDNS found", "peer", pi.ID)
	n.h.Peerstore().AddAddrs(pi.ID, pi.Addrs, peerstore.PermanentAddrTTL)

	if n.h.Network().Connectedness(pi.ID) != network.Connected {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			if err := n.h.Connect(ctx, pi); err != nil {
				slog.Error("mDNS failed to auto-connect", "to", pi.ID, "error", err)
			} else {
				slog.Info("mDNS successfully auto-connected", "to", pi.ID)
			}
		}()
	}
}
