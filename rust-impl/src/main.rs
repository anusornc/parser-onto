use el_reasoner::{saturate, build_taxonomy, count_inferred_subsumptions, AxiomStore};
use std::collections::HashMap;
use std::env;
use std::fs::File;
use std::io::{BufRead, BufReader};
use std::time::Instant;

fn main() {
    let args: Vec<String> = env::args().collect();
    if args.len() < 2 {
        eprintln!("Usage: {} <input.obo>", args[0]);
        std::process::exit(1);
    }

    let input_path = &args[1];
    
    // Parse OBO
    let parse_start = Instant::now();
    let file = File::open(input_path).expect("Failed to open input");
    let reader = BufReader::with_capacity(1024 * 1024, file);
    let parse_result = parse_obo(reader);
    let parse_time = parse_start.elapsed();
    
    let num_concepts = parse_result.concepts.len();
    let num_roles = parse_result.roles.len();
    eprintln!("Parsed {} concepts, {} roles in {:?}", num_concepts, num_roles, parse_time);

    // Build axiom store
    let build_start = Instant::now();
    let store = build_axiom_store(&parse_result);
    let build_time = build_start.elapsed();
    eprintln!("Built axiom store in {:?}", build_time);

    // Saturate
    let sat_start = Instant::now();
    let contexts = saturate(&store, num_concepts, num_roles);
    let sat_time = sat_start.elapsed();
    eprintln!("Saturation complete in {:?}", sat_time);

    // Build taxonomy
    let tax_start = Instant::now();
    let _taxonomy = build_taxonomy(&contexts, num_concepts);
    let tax_time = tax_start.elapsed();
    eprintln!("Taxonomy built in {:?}", tax_time);

    // Count inferred subsumptions
    let inferred = count_inferred_subsumptions(&contexts);

    eprintln!("\n=== Classification Stats ===");
    eprintln!("Concepts: {}", num_concepts - 2);
    eprintln!("Roles: {}", num_roles);
    eprintln!("Inferred subsumptions: {}", inferred);
    eprintln!("Parse time: {:?}", parse_time);
    eprintln!("Normalize time: {:?}", build_time);
    eprintln!("Saturation time: {:?}", sat_time);
    eprintln!("Reduction time: {:?}", tax_time);
    eprintln!("Total time: {:?}", parse_time + build_time + sat_time + tax_time);
}

struct ParseResult {
    concepts: Vec<String>,
    roles: Vec<String>,
    concept_idx: HashMap<String, usize>,
    role_idx: HashMap<String, usize>,
    subsumptions: Vec<(usize, usize)>,
    relations: Vec<(usize, usize, usize)>,
}

fn parse_obo(reader: BufReader<File>) -> ParseResult {
    let mut concepts: Vec<String> = vec!["owl:Thing".to_string(), "owl:Nothing".to_string()];
    let mut roles: Vec<String> = Vec::new();
    let mut concept_idx: HashMap<String, usize> = HashMap::new();
    let mut role_idx: HashMap<String, usize> = HashMap::new();
    
    concept_idx.insert("owl:Thing".to_string(), 0);
    concept_idx.insert("owl:Nothing".to_string(), 1);

    let mut subsumptions: Vec<(usize, usize)> = Vec::new();
    let mut relations: Vec<(usize, usize, usize)> = Vec::new();

    let mut current_id: Option<usize> = None;
    let mut is_obsolete = false;
    let mut in_term = false;

    for line in reader.lines() {
        let line = match line {
            Ok(l) => l,
            Err(_) => continue,
        };
        let line = line.trim();
        
        if line.is_empty() {
            continue;
        }

        if line == "[Term]" {
            in_term = true;
            current_id = None;
            is_obsolete = false;
            continue;
        }

        if line.starts_with("[Typedef]") {
            in_term = false;
            current_id = None;
            continue;
        }

        if line.starts_with('[') {
            in_term = false;
            continue;
        }

        if !in_term {
            continue;
        }

        if line.starts_with("id:") {
            let id = line[3..].trim();
            if let Some(&existing_idx) = concept_idx.get(id) {
                current_id = Some(existing_idx);
            } else {
                let idx = concepts.len();
                concepts.push(id.to_string());
                concept_idx.insert(id.to_string(), idx);
                current_id = Some(idx);
            }
            continue;
        }

        if line.starts_with("is_obsolete:") {
            is_obsolete = line.contains("true");
            continue;
        }

        if is_obsolete {
            continue;
        }

        let Some(sub_idx) = current_id else { continue };

        if line.starts_with("is_a:") {
            let rest = &line[5..];
            let target = rest.split('!').next().unwrap_or("").trim();
            let sup_idx = if let Some(&idx) = concept_idx.get(target) {
                idx
            } else if !target.is_empty() {
                let idx = concepts.len();
                concepts.push(target.to_string());
                concept_idx.insert(target.to_string(), idx);
                idx
            } else {
                continue;
            };
            subsumptions.push((sub_idx, sup_idx));
        } else if line.starts_with("relationship:") {
            let rest = &line[13..];
            let parts: Vec<&str> = rest.split_whitespace().collect();
            if parts.len() >= 2 {
                let role_name = parts[0];
                let target = parts[1];

                let role_idx_val = if let Some(&idx) = role_idx.get(role_name) {
                    idx
                } else {
                    let idx = roles.len();
                    roles.push(role_name.to_string());
                    role_idx.insert(role_name.to_string(), idx);
                    idx
                };

                let target_idx = if let Some(&idx) = concept_idx.get(target) {
                    idx
                } else {
                    let idx = concepts.len();
                    concepts.push(target.to_string());
                    concept_idx.insert(target.to_string(), idx);
                    idx
                };

                relations.push((sub_idx, role_idx_val, target_idx));
            }
        }
    }

    ParseResult {
        concepts,
        roles,
        concept_idx,
        role_idx,
        subsumptions,
        relations,
    }
}

fn build_axiom_store(result: &ParseResult) -> AxiomStore {
    let mut store = AxiomStore::new(result.concepts.len(), result.roles.len());

    for (sub, sup) in &result.subsumptions {
        store.add_subsumption(*sub as u32, *sup as u32);
    }

    for (sub, role, target) in &result.relations {
        store.add_exist_right(*sub as u32, *role as u32, *target as u32);
    }

    store
}
