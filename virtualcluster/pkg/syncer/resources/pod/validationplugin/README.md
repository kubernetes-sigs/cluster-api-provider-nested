# Example Implementations

This is an example of how quota validation plugin should be created and initialized. 

```
package ExampleQuotaValidationPlugin

import validationplugin "sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/resources/pod/validationplugin"

func init() {
        validationplugin.ValidationRegister.Register(&uplugin.Registration{
                ID: validationplugin.QuotaValidationPluginName,
                InitFn: func(ctx *uplugin.InitContext) (interface{}, error) {
                        return NewManager(manager.ResourceSyncerOptions{})
                },
        })
}

type ExampleQuotaValidationPlugin struct {
        validationplugin.QuotaValidationPlugin
}

func NewManager(options manager.ResourceSyncerOptions) (validationplugin.ValidationPluginInterface, error) {
        q := &ExampleQuotaValidationPlugin{QuotaValidationPlugin: *validationplugin.New(validationplugin.QuotaValidationPluginName)}
        go q.CleanIdleTenantInValidationPlugin(q.GetIdleTenantDuration())
        return q, nil
}

func (q *ExampleQuotaValidationPlugin) Validation(podobj client.Object, clusterName string) bool {
        return true
}

```

### Users can extend the validationplugin  interface class and have their own implementations. Following is an example of essential functions that implement per tenant lock and manage tenant list.

```
type QuotaValidationPlugin struct {
   name string   
   alltenants map[string]validationplugin.Tenant   
   *sync.RWMutex   mc     
   *mc.MultiClusterController   
   isFake bool
   }
   
func New(name string) *QuotaValidationPlugin {
    return &QuotaValidationPlugin{
        name:       name,
        alltenants: make(map[string]Tenant),
        RWMutex:    &sync.RWMutex{},
    }
}

func (q *QuotaValidationPlugin) ContextInit(mccontroller *mc.MultiClusterController, isFake bool) {
    q.mc = mccontroller
    q.isFake = isFake
}

func (q *QuotaValidationPlugin) Validation(client.Object, string) bool {
    return true
}

func (q *QuotaValidationPlugin) CleanIdleTenantInValidationPlugin(tenantCheckDuration v1.Duration) {
    m := make(map[string]bool)
    var diff []string
    for {
        time.Sleep(tenantCheckDuration.Duration)
        cnames := q.GetClusterNames()
        if cnames == nil {
            continue
        }
        for _, cn := range cnames {
            m[cn] = true
        }
        q.Lock()
        for k := range q.alltenants {
            if _, ok := m[k]; !ok {
                diff = append(diff, k)
            }
        }
        klog.Infof("delete %v tenant", len(diff))
        for _, k := range diff {
            delete(q.alltenants, k)
            klog.Infof("tenant %v removed from map", k)
        }
        q.Unlock()
    }
}

func (q *QuotaValidationPlugin) GetTenantLocker(clusterName string) *Tenant {
    q.RLock()
    t, ok := q.alltenants[clusterName]
    if ok {
        q.RUnlock()
        return &t
    }
    q.RUnlock()

    q.Lock()
    defer q.Unlock()
    q.alltenants[clusterName] = Tenant{
        ClusterName: clusterName,
        Cond:        &sync.Mutex{},
    }
    if t, ok := q.alltenants[clusterName]; ok {
        klog.V(0).Infof("init lock for tenant %v and then locked", clusterName)
        return &t
    } else {
        klog.Errorf("cannot initialize lock for tenant %v", clusterName)
        return nil
    }
}

func (q *QuotaValidationPlugin) GetClusterNames() []string {
    if q.mc == nil {
        klog.Errorf("mccontroller is nil.")
        return nil
    }
    return q.mc.GetClusterNames()
}

func (q *QuotaValidationPlugin) GetCluster() *mc.MultiClusterController {
    return q.mc
}

func (q *QuotaValidationPlugin) GetName() string {
    return q.name
}

func (q *QuotaValidationPlugin) GetIdleTenantDuration() v1.Duration {
    return v1.Duration{Duration: 120 * time.Minute}
}

func (q *QuotaValidationPlugin) Enabled() bool {
    return false
}
```


