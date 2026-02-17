use fxhash::FxHashMap;
use std::collections::HashSet;

pub type ConceptId = u32;
pub type RoleId = u32;

pub const TOP: ConceptId = 0;
pub const BOTTOM: ConceptId = 1;

#[derive(Clone, Copy, Debug)]
pub struct RoleFiller {
    pub role: RoleId,
    pub fill: ConceptId,
}

#[derive(Clone, Debug, Default)]
pub struct AxiomStore {
    pub sub_to_sups: Vec<Vec<ConceptId>>,
    pub conj_index: Vec<FxHashMap<ConceptId, Vec<ConceptId>>>,
    pub exist_right: Vec<Vec<RoleFiller>>,
    pub exist_left: Vec<FxHashMap<ConceptId, Vec<ConceptId>>>,
}

impl AxiomStore {
    pub fn new(num_concepts: usize, num_roles: usize) -> Self {
        Self {
            sub_to_sups: vec![Vec::new(); num_concepts],
            conj_index: vec![FxHashMap::default(); num_concepts],
            exist_right: vec![Vec::new(); num_concepts],
            exist_left: vec![FxHashMap::default(); num_roles],
        }
    }

    #[inline]
    pub fn add_subsumption(&mut self, sub: ConceptId, sup: ConceptId) {
        self.sub_to_sups[sub as usize].push(sup);
    }

    #[inline]
    pub fn add_exist_right(&mut self, sub: ConceptId, role: RoleId, fill: ConceptId) {
        self.exist_right[sub as usize].push(RoleFiller { role, fill });
    }
}

#[derive(Clone, Debug)]
pub struct Context {
    pub id: ConceptId,
    pub super_set: HashSet<ConceptId>,
    pub link_map: Vec<Vec<ConceptId>>,
    pub pred_map: Vec<Vec<ConceptId>>,
}

impl Context {
    pub fn new(id: ConceptId, num_roles: usize) -> Self {
        Self {
            id,
            super_set: HashSet::with_capacity(16),
            link_map: vec![Vec::new(); num_roles],
            pred_map: vec![Vec::new(); num_roles],
        }
    }
}

#[derive(Clone, Copy, Debug)]
struct WorkItem {
    concept: ConceptId,
    added: ConceptId,
}

#[derive(Clone, Copy, Debug)]
struct LinkItem {
    source: ConceptId,
    role: RoleId,
    target: ConceptId,
}

pub fn saturate(store: &AxiomStore, num_concepts: usize, num_roles: usize) -> Vec<Context> {
    let mut contexts: Vec<Context> = (0..num_concepts)
        .map(|i| Context::new(i as ConceptId, num_roles))
        .collect();

    let mut worklist: Vec<WorkItem> = Vec::with_capacity(num_concepts * 2);
    let mut link_worklist: Vec<LinkItem> = Vec::with_capacity(num_concepts);

    for c in 0..num_concepts {
        let cid = c as ConceptId;
        contexts[c].super_set.insert(cid);
        contexts[c].super_set.insert(TOP);
        worklist.push(WorkItem { concept: cid, added: cid });
        worklist.push(WorkItem { concept: cid, added: TOP });
    }

    while !worklist.is_empty() || !link_worklist.is_empty() {
        while let Some(item) = worklist.pop() {
            let c = item.concept;
            let d = item.added;
            let c_usize = c as usize;
            let d_usize = d as usize;

            // CR1
            if d_usize < store.sub_to_sups.len() {
                for &e in &store.sub_to_sups[d_usize] {
                    if contexts[c_usize].super_set.insert(e) {
                        worklist.push(WorkItem { concept: c, added: e });
                    }
                }
            }

            // CR2
            if d_usize < store.conj_index.len() {
                for (&d2, results) in &store.conj_index[d_usize] {
                    if contexts[c_usize].super_set.contains(&d2) {
                        for &e in results {
                            if contexts[c_usize].super_set.insert(e) {
                                worklist.push(WorkItem { concept: c, added: e });
                            }
                        }
                    }
                }
            }

            // CR3
            if d_usize < store.exist_right.len() {
                for &rf in &store.exist_right[d_usize] {
                    if add_link(&mut contexts, c, rf.fill, rf.role) {
                        link_worklist.push(LinkItem { source: c, role: rf.role, target: rf.fill });
                    }
                }
            }

            // CR4 backward
            for r in 0..num_roles {
                let preds: Vec<ConceptId> = contexts[c_usize].pred_map[r].clone();
                if preds.is_empty() {
                    continue;
                }
                if r >= store.exist_left.len() || store.exist_left[r].is_empty() {
                    continue;
                }
                if let Some(sups) = store.exist_left[r].get(&d) {
                    for &pred in &preds {
                        for &f in sups {
                            if contexts[pred as usize].super_set.insert(f) {
                                worklist.push(WorkItem { concept: pred, added: f });
                            }
                        }
                    }
                }
            }
        }

        while let Some(li) = link_worklist.pop() {
            let c = li.source;
            let r = li.role;
            let d = li.target;
            let c_usize = c as usize;
            let d_usize = d as usize;
            let r_usize = r as usize;

            // CR4 forward
            if r_usize < store.exist_left.len() && !store.exist_left[r_usize].is_empty() {
                let supers: Vec<ConceptId> = contexts[d_usize].super_set.iter().copied().collect();
                for e in supers {
                    if let Some(sups) = store.exist_left[r_usize].get(&e) {
                        for &f in sups {
                            if contexts[c_usize].super_set.insert(f) {
                                worklist.push(WorkItem { concept: c, added: f });
                            }
                        }
                    }
                }
            }

            // CR5
            if contexts[d_usize].super_set.contains(&BOTTOM) {
                if contexts[c_usize].super_set.insert(BOTTOM) {
                    worklist.push(WorkItem { concept: c, added: BOTTOM });
                }
            }

            // CR10 (role subsumption not needed for ChEBI - skip for now)
        }
    }

    contexts
}

#[inline]
fn add_link(contexts: &mut [Context], source: ConceptId, target: ConceptId, role: RoleId) -> bool {
    let source_id = source;
    let target_id = target;
    
    for &existing in &contexts[source as usize].link_map[role as usize] {
        if existing == target_id {
            return false;
        }
    }
    contexts[source as usize].link_map[role as usize].push(target_id);
    contexts[target as usize].pred_map[role as usize].push(source_id);
    true
}

pub fn build_taxonomy(contexts: &[Context], num_concepts: usize) -> Vec<Vec<ConceptId>> {
    let mut direct_parents: Vec<Vec<ConceptId>> = vec![Vec::new(); num_concepts];

    for c in 2..num_concepts {
        let supers = &contexts[c].super_set;

        let mut candidates: Vec<ConceptId> = Vec::with_capacity(supers.len());
        let mut has_top = false;

        for &s in supers {
            match s {
                TOP => { has_top = true; continue; }
                BOTTOM => continue,
                x if x == c as ConceptId => continue,
                _ => candidates.push(s),
            }
        }

        let mut direct: Vec<ConceptId> = Vec::with_capacity(4);
        'outer: for &b in &candidates {
            for &s in &candidates {
                if s == b {
                    continue;
                }
                if contexts[s as usize].super_set.contains(&b) {
                    continue 'outer;
                }
            }
            direct.push(b);
        }

        if direct.is_empty() && has_top {
            direct.push(TOP);
        }

        direct_parents[c] = direct;
    }

    direct_parents
}

pub fn count_inferred_subsumptions(contexts: &[Context]) -> usize {
    contexts.iter()
        .skip(2)
        .map(|c| c.super_set.len().saturating_sub(2))
        .sum()
}
