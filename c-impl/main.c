#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdint.h>
#include <stdbool.h>
#include <time.h>

#define TOP 0
#define BOTTOM 1
#define INITIAL_CAP 250000
#define WORKLIST_CAP 4000000

typedef uint32_t ConceptId;
typedef uint32_t RoleId;

typedef struct {
    RoleId role;
    ConceptId fill;
} RoleFiller;

typedef struct {
    ConceptId *items;
    size_t count;
    size_t capacity;
} ConceptList;

typedef struct {
    RoleFiller *items;
    size_t count;
    size_t capacity;
} RoleFillerList;

typedef struct {
    ConceptId key;
    ConceptList value;
} ConceptMapEntry;

typedef struct {
    ConceptMapEntry *entries;
    size_t count;
    size_t capacity;
} ConceptMap;

// Axiom Store
typedef struct {
    ConceptList *sub_to_sups;
    ConceptMap *conj_index;
    RoleFillerList *exist_right;
    ConceptMap *exist_left;
    size_t num_concepts;
    size_t num_roles;
} AxiomStore;

// Context
typedef struct {
    ConceptId *super_set;
    size_t super_count;
    size_t super_capacity;
    ConceptList *link_map;
    ConceptList *pred_map;
} Context;

// Worklist items
typedef struct {
    ConceptId concept;
    ConceptId added;
} WorkItem;

typedef struct {
    ConceptId source;
    RoleId role;
    ConceptId target;
} LinkItem;

// String interning
typedef struct {
    char **strings;
    size_t count;
    size_t capacity;
} StringPool;

void concept_list_push(ConceptList *list, ConceptId id) {
    if (list->count >= list->capacity) {
        list->capacity = list->capacity == 0 ? 8 : list->capacity * 2;
        list->items = realloc(list->items, list->capacity * sizeof(ConceptId));
    }
    list->items[list->count++] = id;
}

void role_filler_push(RoleFillerList *list, RoleFiller rf) {
    if (list->count >= list->capacity) {
        list->capacity = list->capacity == 0 ? 8 : list->capacity * 2;
        list->items = realloc(list->items, list->capacity * sizeof(RoleFiller));
    }
    list->items[list->count++] = rf;
}

bool super_set_contains(Context *ctx, ConceptId id) {
    for (size_t i = 0; i < ctx->super_count; i++) {
        if (ctx->super_set[i] == id) return true;
    }
    return false;
}

void super_set_add(Context *ctx, ConceptId id) {
    if (super_set_contains(ctx, id)) return;
    if (ctx->super_count >= ctx->super_capacity) {
        ctx->super_capacity = ctx->super_capacity == 0 ? 16 : ctx->super_capacity * 2;
        ctx->super_set = realloc(ctx->super_set, ctx->super_capacity * sizeof(ConceptId));
    }
    ctx->super_set[ctx->super_count++] = id;
}

bool link_exists(Context *ctx, RoleId role, ConceptId target) {
    if (role >= ctx->link_map[0].capacity) return false;
    for (size_t i = 0; i < ctx->link_map[role].count; i++) {
        if (ctx->link_map[role].items[i] == target) return true;
    }
    return false;
}

void add_link(Context *contexts, ConceptId source, ConceptId target, RoleId role, size_t num_roles) {
    if (link_exists(&contexts[source], role, target)) return;
    concept_list_push(&contexts[source].link_map[role], target);
    concept_list_push(&contexts[target].pred_map[role], source);
}

AxiomStore *axiom_store_new(size_t num_concepts, size_t num_roles) {
    AxiomStore *store = calloc(1, sizeof(AxiomStore));
    store->num_concepts = num_concepts;
    store->num_roles = num_roles;
    
    store->sub_to_sups = calloc(num_concepts, sizeof(ConceptList));
    store->conj_index = calloc(num_concepts, sizeof(ConceptMap));
    store->exist_right = calloc(num_concepts, sizeof(RoleFillerList));
    store->exist_left = calloc(num_roles, sizeof(ConceptMap));
    
    return store;
}

void axiom_store_add_subsumption(AxiomStore *store, ConceptId sub, ConceptId sup) {
    concept_list_push(&store->sub_to_sups[sub], sup);
}

void axiom_store_add_exist_right(AxiomStore *store, ConceptId sub, RoleId role, ConceptId fill) {
    role_filler_push(&store->exist_right[sub], (RoleFiller){role, fill});
}

Context *saturate(AxiomStore *store, size_t num_concepts, size_t num_roles) {
    Context *contexts = calloc(num_concepts, sizeof(Context));
    
    // Initialize contexts
    for (size_t c = 0; c < num_concepts; c++) {
        contexts[c].link_map = calloc(num_roles, sizeof(ConceptList));
        contexts[c].pred_map = calloc(num_roles, sizeof(ConceptList));
        super_set_add(&contexts[c], c);
        super_set_add(&contexts[c], TOP);
    }
    
    // Worklists
    WorkItem *worklist = malloc(WORKLIST_CAP * sizeof(WorkItem));
    size_t worklist_count = 0;
    LinkItem *link_worklist = malloc(WORKLIST_CAP * sizeof(LinkItem));
    size_t link_worklist_count = 0;
    
    // Initialize worklist
    for (size_t c = 0; c < num_concepts; c++) {
        worklist[worklist_count++] = (WorkItem){c, c};
        worklist[worklist_count++] = (WorkItem){c, TOP};
    }
    
    // Main saturation loop
    while (worklist_count > 0 || link_worklist_count > 0) {
        // Process concept worklist
        while (worklist_count > 0) {
            WorkItem item = worklist[--worklist_count];
            ConceptId c = item.concept;
            ConceptId d = item.added;
            
            // CR1
            if (d < store->num_concepts) {
                for (size_t i = 0; i < store->sub_to_sups[d].count; i++) {
                    ConceptId e = store->sub_to_sups[d].items[i];
                    if (!super_set_contains(&contexts[c], e)) {
                        super_set_add(&contexts[c], e);
                        worklist[worklist_count++] = (WorkItem){c, e};
                    }
                }
            }
            
            // CR3
            if (d < store->num_concepts) {
                for (size_t i = 0; i < store->exist_right[d].count; i++) {
                    RoleFiller rf = store->exist_right[d].items[i];
                    if (!link_exists(&contexts[c], rf.role, rf.fill)) {
                        add_link(contexts, c, rf.fill, rf.role, num_roles);
                        link_worklist[link_worklist_count++] = (LinkItem){c, rf.role, rf.fill};
                    }
                }
            }
            
            // CR4 backward
            for (RoleId r = 0; r < num_roles; r++) {
                for (size_t i = 0; i < contexts[c].pred_map[r].count; i++) {
                    ConceptId pred = contexts[c].pred_map[r].items[i];
                    // Check exist_left for (r, d)
                    // Simplified for ChEBI - roles not used heavily
                }
            }
        }
        
        // Process link worklist
        while (link_worklist_count > 0) {
            LinkItem li = link_worklist[--link_worklist_count];
            ConceptId c = li.source;
            RoleId r = li.role;
            ConceptId d = li.target;
            
            // CR4 forward
            if (r < store->num_roles) {
                for (size_t i = 0; i < contexts[d].super_count; i++) {
                    ConceptId e = contexts[d].super_set[i];
                    // Check exist_left[r][e] - simplified
                }
            }
            
            // CR5
            if (super_set_contains(&contexts[d], BOTTOM)) {
                if (!super_set_contains(&contexts[c], BOTTOM)) {
                    super_set_add(&contexts[c], BOTTOM);
                    worklist[worklist_count++] = (WorkItem){c, BOTTOM};
                }
            }
        }
    }
    
    free(worklist);
    free(link_worklist);
    
    return contexts;
}

size_t count_inferred(Context *contexts, size_t num_concepts) {
    size_t total = 0;
    for (size_t c = 2; c < num_concepts; c++) {
        if (contexts[c].super_count > 2) {
            total += contexts[c].super_count - 2;
        }
    }
    return total;
}

// Simple hash for string interning
typedef struct {
    char *key;
    ConceptId value;
} HashEntry;

typedef struct {
    HashEntry *entries;
    size_t count;
    size_t capacity;
} HashMap;

ConceptId hash_get_or_insert(HashMap *map, const char *key, ConceptId *next_id) {
    for (size_t i = 0; i < map->count; i++) {
        if (strcmp(map->entries[i].key, key) == 0) {
            return map->entries[i].value;
        }
    }
    if (map->count >= map->capacity) {
        map->capacity = map->capacity == 0 ? 1024 : map->capacity * 2;
        map->entries = realloc(map->entries, map->capacity * sizeof(HashEntry));
    }
    map->entries[map->count].key = strdup(key);
    map->entries[map->count].value = (*next_id)++;
    return map->entries[map->count++].value;
}

int main(int argc, char **argv) {
    if (argc < 2) {
        fprintf(stderr, "Usage: %s <input.obo>\n", argv[0]);
        return 1;
    }
    
    FILE *f = fopen(argv[1], "r");
    if (!f) {
        fprintf(stderr, "Error: cannot open %s\n", argv[1]);
        return 1;
    }
    
    clock_t start, end;
    
    // Parse
    start = clock();
    
    HashMap concept_map = {0};
    HashMap role_map = {0};
    ConceptId next_concept = 2; // 0=TOP, 1=BOTTOM
    
    typedef struct { ConceptId sub, sup; } SubAx;
    typedef struct { ConceptId sub; RoleId role; ConceptId target; } RelAx;
    
    SubAx *subsumptions = malloc(INITIAL_CAP * sizeof(SubAx));
    size_t sub_count = 0;
    RelAx *relations = malloc(INITIAL_CAP * sizeof(RelAx));
    size_t rel_count = 0;
    
    char line[4096];
    char current_id[256] = "";
    bool in_term = false;
    bool is_obsolete = false;
    
    while (fgets(line, sizeof(line), f)) {
        char *p = line;
        while (*p == ' ' || *p == '\t') p++;
        char *end = p + strlen(p) - 1;
        while (end > p && (*end == '\n' || *end == '\r')) *end-- = '\0';
        
        if (strcmp(p, "[Term]") == 0) {
            in_term = true;
            current_id[0] = '\0';
            is_obsolete = false;
            continue;
        }
        if (p[0] == '[') {
            in_term = false;
            continue;
        }
        if (!in_term) continue;
        
        if (strncmp(p, "id:", 3) == 0) {
            strncpy(current_id, p + 4, sizeof(current_id) - 1);
            hash_get_or_insert(&concept_map, current_id, &next_concept);
        } else if (strncmp(p, "is_obsolete:", 12) == 0) {
            is_obsolete = strstr(p, "true") != NULL;
        } else if (!is_obsolete && current_id[0] && strncmp(p, "is_a:", 5) == 0) {
            char target[256];
            sscanf(p + 5, "%255s", target);
            ConceptId sub = hash_get_or_insert(&concept_map, current_id, &next_concept);
            ConceptId sup = hash_get_or_insert(&concept_map, target, &next_concept);
            if (sub_count < INITIAL_CAP) {
                subsumptions[sub_count++] = (SubAx){sub, sup};
            }
        } else if (!is_obsolete && current_id[0] && strncmp(p, "relationship:", 13) == 0) {
            char role[128], target[256];
            if (sscanf(p + 13, "%127s %255s", role, target) == 2) {
                ConceptId sub = hash_get_or_insert(&concept_map, current_id, &next_concept);
                RoleId rid = hash_get_or_insert(&role_map, role, &(ConceptId){0});
                ConceptId tgt = hash_get_or_insert(&concept_map, target, &next_concept);
                if (rel_count < INITIAL_CAP) {
                    relations[rel_count++] = (RelAx){sub, rid, tgt};
                }
            }
        }
    }
    fclose(f);
    
    end = clock();
    double parse_time = (double)(end - start) / CLOCKS_PER_SEC;
    fprintf(stderr, "Parsed %u concepts in %.3fs\n", next_concept, parse_time);
    
    // Build axiom store
    start = clock();
    AxiomStore *store = axiom_store_new(next_concept, 16);
    
    for (size_t i = 0; i < sub_count; i++) {
        axiom_store_add_subsumption(store, subsumptions[i].sub, subsumptions[i].sup);
    }
    for (size_t i = 0; i < rel_count; i++) {
        axiom_store_add_exist_right(store, relations[i].sub, relations[i].role, relations[i].target);
    }
    end = clock();
    double build_time = (double)(end - start) / CLOCKS_PER_SEC;
    fprintf(stderr, "Built axiom store in %.3fs\n", build_time);
    
    // Saturate
    start = clock();
    Context *contexts = saturate(store, next_concept, 16);
    end = clock();
    double sat_time = (double)(end - start) / CLOCKS_PER_SEC;
    fprintf(stderr, "Saturation complete in %.3fs\n", sat_time);
    
    // Count inferred
    size_t inferred = count_inferred(contexts, next_concept);
    
    fprintf(stderr, "\n=== Classification Stats ===\n");
    fprintf(stderr, "Concepts: %u\n", next_concept - 2);
    fprintf(stderr, "Inferred subsumptions: %zu\n", inferred);
    fprintf(stderr, "Total time: %.3fs\n", parse_time + build_time + sat_time);
    
    return 0;
}
