package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"time"

	// "github.com/golang/glog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	"github.com/Thoro/bfd/pkg/api"
	"github.com/spf13/cobra"
)

var version = "master"
var cancel context.CancelFunc
var client api.BfdApiClient

var peer string

const (
	cmdPeers                    = "peers"
	cmdEnable                   = "enable"
	cmdDisable                  = "disable"
	cmdSet                      = "set"
	cmdSetDesiredMinTxInterval  = "DesiredMinTxInterval"
	cmdSetRequiredMinRxInterval = "RequiredMinRxInterval"
	cmdSetDetectMultiplier      = "DetectMultiplier"
	cmdAdd                      = "add"
	cmdDel                      = "del"
	cmdMonitor                  = "monitor"
)

type options struct {
	TLS    bool
	CaFile string
	Port   int
	Host   string
}

func main() {
	newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	cobra.EnablePrefixMatching = true

	var err error

	ctx := context.Background()
	opts := &options{}

	rootCmd := &cobra.Command{
		Use: "bfd",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			client, cancel, err = newClient(ctx, opts)

			if err != nil {
				exitWithError(errors.New(fmt.Sprintf("Connection to grpc failed: %s", err)))
			}
		},
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if cancel != nil {
				cancel()
			}
		},
	}

	rootCmd.PersistentFlags().StringVarP(&opts.Host, "host", "", "127.0.0.1", "GRPC host")
	rootCmd.PersistentFlags().IntVarP(&opts.Port, "port", "", api.GRPC_PORT, "GRPC port")
	rootCmd.PersistentFlags().BoolVarP(&opts.TLS, "tls", "", false, "Enable TLS for grpc")
	rootCmd.PersistentFlags().StringVarP(&opts.CaFile, "tls-ca-file", "", "", "The file containing the CA root cert")

	rootCmd.AddCommand(newPeerCmd())
	rootCmd.AddCommand(addRequiredFlag(newMonitorCmd(), true))

	return rootCmd
}

func getPeers() (map[string][]byte, error) {
	peers := make(map[string][]byte, 0)
	stream, err := client.ListPeer(context.Background(), &api.ListPeerRequest{})

	if err != nil {
		return nil, err
	}

	for {
		response, err := stream.Recv()

		if err == io.EOF {
			break
		}

		peer := response.Peer
		peers[peer.Name] = response.Uuid
		peers[peer.Address] = response.Uuid
	}

	return peers, nil
}

func newPeerCmd() *cobra.Command {
	peerSetCmd := &cobra.Command{
		Use: "set",
	}

	peerSetCmd.AddCommand(newPeerSetDesiredMinTxIntervalCmd())
	peerSetCmd.AddCommand(newPeerSetRequiredMinRxIntervalCmd())
	peerSetCmd.AddCommand(newPeerSetDetectMultiplierCmd())

	peers := &cobra.Command{
		Use:  cmdPeers,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			stream, err := client.ListPeer(context.Background(), &api.ListPeerRequest{})

			if err != nil {
				return
			}

			count := 0

			for {
				response, err := stream.Recv()

				if err == io.EOF {
					break
				}

				if err != nil {
					fmt.Printf("Error Listing peers: %s", err.Error())
					return
				}

				state, err := client.GetPeerState(context.Background(), &api.GetPeerStateRequest{
					Uuid: response.Uuid,
				})

				count++

				peer := response.Peer

				fmt.Printf("%s\t%s\t%s <-> %s 127.0.0.1\n", peer.Name, peer.Address, state.Remote.State, state.Local.State)
			}

			if count == 0 {
				fmt.Printf("No peers found.\n")
			}
		},
	}

	addRequiredFlag(peers, false)

	peers.AddCommand(addRequiredFlag(peerSetCmd, true))
	peers.AddCommand(newPeerAddCmd())
	peers.AddCommand(addRequiredFlag(newPeerDelCmd(), true))
	peers.AddCommand(addRequiredFlag(newPeerEnableCmd(), true))
	peers.AddCommand(addRequiredFlag(newPeerDisableCmd(), true))

	return peers
}

func addRequiredFlag(cmd *cobra.Command, persistent bool) *cobra.Command {
	cmd.PersistentFlags().StringVarP(&peer, "peer", "p", "", "peer that should be modified")

	if persistent {
		cmd.MarkPersistentFlagRequired("peer")
	}

	return cmd
}

func newPeerSetDesiredMinTxIntervalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: cmdSetDesiredMinTxInterval,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("Need to pass the new value.")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			peers, _ := getPeers()

			interval, err := strconv.ParseUint(args[0], 10, 32)

			if err != nil {
				fmt.Printf("Error parsing parameter: %s\n", err.Error())
				return
			}

			if uuid, ok := peers[peer]; ok {
				_, err := client.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
					Uuid: uuid,
					Peer: &api.Peer{
						DesiredMinTxInterval: uint32(interval),
					},
				})

				if err != nil {
					fmt.Printf("Error updating peer: %s\n", err.Error())
				} else {
					fmt.Printf("Updated peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return cmd
}

func newPeerSetRequiredMinRxIntervalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: cmdSetRequiredMinRxInterval,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("Need to pass the new value.")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			peers, _ := getPeers()

			interval, err := strconv.ParseUint(args[0], 10, 32)

			if err != nil {
				fmt.Printf("Error parsing parameter: %s\n", err.Error())
				return
			}

			if uuid, ok := peers[peer]; ok {
				_, err := client.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
					Uuid: uuid,
					Peer: &api.Peer{
						RequiredMinRxInterval: uint32(interval),
					},
				})

				if err != nil {
					fmt.Printf("Error updating peer: %s\n", err.Error())
				} else {
					fmt.Printf("Updated peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return cmd
}

func newPeerSetDetectMultiplierCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: cmdSetDetectMultiplier,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("Need to pass the new value.")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {

			peers, _ := getPeers()

			interval, err := strconv.ParseUint(args[0], 10, 8)

			if err != nil {
				fmt.Printf("Error parsing parameter: %s\n", err.Error())
				return
			}

			if uuid, ok := peers[peer]; ok {
				_, err := client.UpdatePeer(context.Background(), &api.UpdatePeerRequest{
					Uuid: uuid,
					Peer: &api.Peer{
						DetectMultiplier: uint32(interval),
					},
				})

				if err != nil {
					fmt.Printf("Error updating peer: %s\n", err.Error())
				} else {
					fmt.Printf("Updated peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return cmd
}

func newPeerAddCmd() *cobra.Command {
	var ip net.IP
	var txinterval, rxinterval, multiplier uint64

	cmd := &cobra.Command{
		Use: cmdAdd,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 7 {
				return errors.New("Please pass the following parameters: {Name} {IP} {DesiredMinTxInterval} {RequiredMinRxInterval} {DetectMultiplier} {IsMultiHop[Yes|No]} {Authentication[None|SimplePassword|KeyedMD5|MeticulousKeyedMD5|KeyedSHA1|MeticulousKeyedSHA1]} {PasswordIfAuthenticationIsNotNone}")
			}

			var err error

			ip = net.ParseIP(args[1])

			if ip == nil {
				return errors.New("Please pass a valid ip address")
			}

			txinterval, err = strconv.ParseUint(args[2], 10, 32)

			if err != nil {
				return errors.New(fmt.Sprintf("Error parsing DesiredMinTxInterval: %s", err.Error()))
			}

			rxinterval, err = strconv.ParseUint(args[3], 10, 32)

			if err != nil {
				return errors.New(fmt.Sprintf("Error parsing RequiredMinRxInterval: %s", err.Error()))
			}

			multiplier, err = strconv.ParseUint(args[4], 10, 8)

			if err != nil {
				return errors.New(fmt.Sprintf("Error parsing DetectMultiplier: %s", err.Error()))
			}

			if args[5] != "No" {
				return errors.New("Only No is supported for IsMultiHop")
			}

			if args[6] != "None" {
				return errors.New("Only None is suppored for Authentication")
			}

			return nil
		},
		Run: func(cmd *cobra.Command, args []string) {
			_, err := client.AddPeer(context.Background(), &api.AddPeerRequest{
				Peer: &api.Peer{
					Name:                  args[0],
					Address:               ip.String(),
					DesiredMinTxInterval:  uint32(txinterval),
					RequiredMinRxInterval: uint32(rxinterval),
					DetectMultiplier:      uint32(multiplier),
				},
			})

			if err != nil {
				fmt.Printf("Error adding peer %s\n", err.Error())
			} else {
				fmt.Printf("Added peer %s\n", args[0])
			}
		},
	}

	return cmd
}

func newPeerDelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:  cmdDel,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {

			peers, _ := getPeers()

			if uuid, ok := peers[peer]; ok {
				_, err := client.DeletePeer(context.Background(), &api.DeletePeerRequest{
					Uuid: uuid,
				})

				if err != nil {
					fmt.Printf("Error deleting peer: %s\n", err.Error())
				} else {
					fmt.Printf("Deleted peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return cmd
}

func newPeerEnableCmd() *cobra.Command {
	enable := &cobra.Command{
		Use:  cmdEnable,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			peers, _ := getPeers()

			if uuid, ok := peers[peer]; ok {
				_, err := client.EnablePeer(context.Background(), &api.EnablePeerRequest{
					Uuid: uuid,
				})

				if err != nil {
					fmt.Printf("Error enabling peer: %s\n", err.Error())
				} else {
					fmt.Printf("Enabled peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return enable
}

func newPeerDisableCmd() *cobra.Command {
	disable := &cobra.Command{
		Use:  cmdDisable,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			peers, _ := getPeers()

			if uuid, ok := peers[peer]; ok {
				_, err := client.DisablePeer(context.Background(), &api.DisablePeerRequest{
					Uuid: uuid,
				})

				if err != nil {
					fmt.Printf("Error disabling peer: %s\n", err.Error())
				} else {
					fmt.Printf("Disabled peer %s\n", peer)
				}
			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return disable
}

func newMonitorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: cmdMonitor,
		Run: func(cmd *cobra.Command, args []string) {
			peers, _ := getPeers()

			if uuid, ok := peers[peer]; ok {
				stream, _ := client.MonitorPeer(
					context.Background(),
					&api.MonitorPeerRequest{
						Uuid: uuid,
					},
				)

				for {
					response, err := stream.Recv()

					if err == io.EOF {
						break
					}

					if err != nil {
						fmt.Printf("Error monitoring peer: %s\n", err.Error())
						return
					}

					fmt.Printf("[%s] %s <-> %s\n", time.Now().Format(time.RFC3339), response.Local.State.String(), response.Remote.State.String())
				}

			} else {
				fmt.Printf("Peer not found!\n")
			}
		},
	}

	return cmd
}

func exitWithError(err error) {
	printError(err)
	os.Exit(1)
}

func printError(err error) {
	fmt.Println(err)
}

func newClient(ctx context.Context, opts *options) (api.BfdApiClient, context.CancelFunc, error) {
	grpcOpts := []grpc.DialOption{grpc.WithBlock()}

	if opts.TLS {
		var creds credentials.TransportCredentials

		if opts.CaFile == "" {
			creds = credentials.NewClientTLSFromCert(nil, "")
		} else {
			var err error
			creds, err = credentials.NewClientTLSFromFile(opts.CaFile, "")

			if err != nil {
				return nil, nil, err
			}
		}

		grpcOpts = append(grpcOpts, grpc.WithTransportCredentials(creds))
	} else {
		grpcOpts = append(grpcOpts, grpc.WithInsecure())
	}

	target := net.JoinHostPort(opts.Host, strconv.Itoa(opts.Port))

	if target == "" {
		target = ":" + strconv.Itoa(api.GRPC_PORT)
	}

	cc, cancel := context.WithTimeout(ctx, time.Second)
	conn, err := grpc.DialContext(cc, target, grpcOpts...)

	if err != nil {
		return nil, cancel, err
	}

	return api.NewBfdApiClient(conn), cancel, nil
}
