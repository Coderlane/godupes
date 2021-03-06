package main

// #include <btrfs/ioctl.h>
// void same_arg_set_extent_info(struct btrfs_ioctl_same_args* arg, int i, int fd, int offset) {
//   struct btrfs_ioctl_same_extent_info *info = &arg->info[i];
//   info->fd = fd;
//   info->logical_offset = offset;
// }
// void same_arg_get_extent_info(struct btrfs_ioctl_same_args* arg, int i, int *bytes_deduped, int *status) {
//   struct btrfs_ioctl_same_extent_info *info = &arg->info[i];
//   *bytes_deduped = info->bytes_deduped;
//   *status = info->status;
// }
import "C"

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"

	cli "github.com/urfave/cli/v2"
)

// parse the output of the 'fdupes' program
func parseFdupesFile(infi string) ([][]string, error) {
	fi, err := os.Open(infi)
	if err != nil {
		return nil, err
	}

	var out [][]string
	var cur []string
	scan := bufio.NewScanner(fi)
	for scan.Scan() {
		if len(scan.Text()) == 0 {
			if len(cur) > 0 {
				out = append(out, cur)
				cur = nil
			}
		} else {
			cur = append(cur, scan.Text())
		}
	}
	if len(cur) > 0 {
		out = append(out, cur)
	}

	return out, nil
}

func main() {
	app := cli.NewApp()
	app.Commands = []*cli.Command{
		runDedupeCmd,
		findDupesCmd,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Println(err)
	}
}

var runDedupeCmd = &cli.Command{
	Name: "run-dedupe",
	Action: func(cctx *cli.Context) error {
		input, err := parseFdupesFile(cctx.Args().First())
		if err != nil {
			return err
		}

		for _, batch := range input {
			fmt.Println("Deduplicating: ", batch[0])
			for _, o := range batch[1:] {
				fmt.Printf("\t%s\n", o)
			}
			if err := DedupeFiles(batch); err != nil {
				return err
			}
		}

		return nil
	},
}

var findDupesCmd = &cli.Command{
	Name:  "find-dupes",
	Usage: "find duplicate files in the given directory",
	Action: func(cctx *cli.Context) error {
		var filelist []string

		root := cctx.Args().First()
		// TODO: walk isnt super efficient, and doesnt let us do anything in parallel
		err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("walk function errored at %q: %w", p, err)
			}
			if info.Mode().IsRegular() {
				filelist = append(filelist, p)
			}
			return nil
		})
		if err != nil {
			return err
		}

		// TODO: caching the entire list of files on disk might be useful. On
		// large systems this takes a long time to generate

		lookup := make(map[string][]string)

		for _, f := range filelist {
			hkey, err := hashFile(f)
			if err != nil {
				return fmt.Errorf("failed to hash file %q: %w", f, err)
			}

			hks := string(hkey)
			lookup[hks] = append(lookup[hks], f)
		}

		for _, matches := range lookup {
			if len(matches) > 1 {
				for _, m := range matches {
					absp, err := filepath.Abs(m)
					if err != nil {
						return fmt.Errorf("failed to get absolute path for %q: %w", m, err)
					}

					fmt.Println(absp)
				}
				fmt.Println()
			}
		}

		return nil
	},
}

func hashFile(f string) ([]byte, error) {
	h := sha256.New()

	fi, err := os.Open(f)
	if err != nil {
		return nil, err
	}

	_, err = io.Copy(h, fi)
	if err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

func DedupeFiles(fis []string) error {
	f1, err := os.Open(os.Args[1])
	if err != nil {
		return err
	}
	defer f1.Close()

	f1s, err := f1.Stat()
	if err != nil {
		return err
	}

	size := unsafe.Sizeof(C.struct_btrfs_ioctl_same_args{})
	size += unsafe.Sizeof(C.struct_btrfs_ioctl_same_extent_info{}) * 1
	arg := (*C.struct_btrfs_ioctl_same_args)(C.malloc(C.ulong(size)))

	arg.length = C.ulonglong(f1s.Size())
	arg.dest_count = C.ushort(len(fis) - 1)

	for i := 0; i < len(fis)-1; i++ {
		destfi, err := os.Open(os.Args[2])
		if err != nil {
			return err
		}
		defer destfi.Close()

		C.same_arg_set_extent_info(arg, C.int(i), C.int(destfi.Fd()), 0)
	}

	r1, r2, errno := syscall.Syscall(syscall.SYS_IOCTL, f1.Fd(), C.BTRFS_IOC_FILE_EXTENT_SAME, uintptr(unsafe.Pointer(arg)))
	fmt.Println("syscall ret: ", r1, int(r2), errno)
	for i := 0; i < int(arg.dest_count); i++ {
		var (
			status       C.int
			bytesDeduped C.int
		)
		C.same_arg_get_extent_info(arg, C.int(i), &bytesDeduped, &status)
		fmt.Println("resp status: ", status)
		fmt.Println("resp deduped: ", bytesDeduped)
	}
	return nil
}
