package ovirt

import (
	"path"
	"strings"

	api "github.com/kubev2v/forklift/pkg/apis/forklift/v1beta1"
	"github.com/kubev2v/forklift/pkg/controller/provider/web/ovirt"
	"github.com/kubev2v/forklift/pkg/controller/watch/handler"
	liberr "github.com/kubev2v/forklift/pkg/lib/error"
	libweb "github.com/kubev2v/forklift/pkg/lib/inventory/web"
	"github.com/kubev2v/forklift/pkg/lib/logging"
	"golang.org/x/net/context"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// Package logger.
var log = logging.WithName("storageMap|ovirt")

// Provider watch event handler.
type Handler struct {
	*handler.Handler
}

// Ensure watch on StorageDomain.
func (r *Handler) Watch(watch *handler.WatchManager) (err error) {
	w, err := watch.Ensure(
		r.Provider(),
		&ovirt.StorageDomain{},
		r)
	if err != nil {
		return
	}

	log.Info(
		"Inventory watch ensured.",
		"provider",
		path.Join(
			r.Provider().Namespace,
			r.Provider().Name),
		"watch",
		w.ID())

	return
}

// Resource created.
func (r *Handler) Created(e libweb.Event) {
	if ds, cast := e.Resource.(*ovirt.StorageDomain); cast {
		r.changed(ds)
	}
}

// Resource created.
func (r *Handler) Updated(e libweb.Event) {
	if ds, cast := e.Resource.(*ovirt.StorageDomain); cast {
		updated := e.Updated.(*ovirt.StorageDomain)
		if updated.Path != ds.Path {
			r.changed(ds, updated)
		}
	}
}

// Resource deleted.
func (r *Handler) Deleted(e libweb.Event) {
	if ds, cast := e.Resource.(*ovirt.StorageDomain); cast {
		r.changed(ds)
	}
}

// Storage changed.
// Find all of the StorageMap CRs the reference both the
// provider and the changed storage domain and enqueue reconcile events.
func (r *Handler) changed(models ...*ovirt.StorageDomain) {
	log.V(3).Info(
		"Storage domain changed.",
		"id",
		models[0].ID)
	list := api.StorageMapList{}
	err := r.List(context.TODO(), &list)
	if err != nil {
		err = liberr.Wrap(err)
		log.Error(err, "failed to list StorageMap CRs")
		return
	}
	for i := range list.Items {
		mp := &list.Items[i]
		ref := mp.Spec.Provider.Source
		if !r.MatchProvider(ref) {
			continue
		}
		referenced := false
		for _, pair := range mp.Spec.Map {
			ref := pair.Source
			for _, ds := range models {
				if ref.ID == ds.ID || strings.HasSuffix(ds.Path, ref.Name) {
					referenced = true
					break
				}
			}
			if referenced {
				break
			}
		}
		if referenced {
			log.V(3).Info(
				"Queue reconcile event.",
				"map",
				path.Join(
					mp.Namespace,
					mp.Name))
			r.Enqueue(event.GenericEvent{
				Object: mp,
			})
		}
	}
}
