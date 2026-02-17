#include <iostream>
#include <fstream>
#include <sstream>
#include <vector>
#include <unordered_map>
#include <unordered_set>
#include <string>
#include <chrono>
#include <cstdint>

using ConceptId = uint32_t;
using RoleId = uint32_t;

constexpr ConceptId TOP = 0;
constexpr ConceptId BOTTOM = 1;

struct RoleFiller {
    RoleId role;
    ConceptId fill;
};

class AxiomStore {
public:
    std::vector<std::vector<ConceptId>> sub_to_sups;
    std::vector<std::unordered_map<ConceptId, std::vector<ConceptId>>> conj_index;
    std::vector<std::vector<RoleFiller>> exist_right;
    std::vector<std::unordered_map<ConceptId, std::vector<ConceptId>>> exist_left;
    
    AxiomStore(size_t num_concepts, size_t num_roles) {
        sub_to_sups.resize(num_concepts);
        conj_index.resize(num_concepts);
        exist_right.resize(num_concepts);
        exist_left.resize(num_roles);
    }
    
    void addSubsumption(ConceptId sub, ConceptId sup) {
        sub_to_sups[sub].push_back(sup);
    }
    
    void addExistRight(ConceptId sub, RoleId role, ConceptId fill) {
        exist_right[sub].push_back({role, fill});
    }
};

class Context {
public:
    ConceptId id;
    std::unordered_set<ConceptId> super_set;
    std::vector<std::vector<ConceptId>> link_map;
    std::vector<std::vector<ConceptId>> pred_map;
    
    Context(ConceptId id, size_t num_roles) : id(id) {
        link_map.resize(num_roles);
        pred_map.resize(num_roles);
    }
    
    bool hasSuper(ConceptId c) const {
        return super_set.count(c) > 0;
    }
    
    void addSuper(ConceptId c) {
        super_set.insert(c);
    }
    
    bool hasLink(RoleId role, ConceptId target) const {
        for (ConceptId t : link_map[role]) {
            if (t == target) return true;
        }
        return false;
    }
};

struct WorkItem {
    ConceptId concept;
    ConceptId added;
};

struct LinkItem {
    ConceptId source;
    RoleId role;
    ConceptId target;
};

std::vector<Context> saturate(const AxiomStore& store, size_t num_concepts, size_t num_roles) {
    std::vector<Context> contexts;
    contexts.reserve(num_concepts);
    for (size_t c = 0; c < num_concepts; c++) {
        contexts.emplace_back(c, num_roles);
        contexts[c].addSuper(c);
        contexts[c].addSuper(TOP);
    }
    
    std::vector<WorkItem> worklist;
    worklist.reserve(num_concepts * 2);
    std::vector<LinkItem> link_worklist;
    link_worklist.reserve(num_concepts);
    
    for (size_t c = 0; c < num_concepts; c++) {
        worklist.push_back({static_cast<ConceptId>(c), static_cast<ConceptId>(c)});
        worklist.push_back({static_cast<ConceptId>(c), TOP});
    }
    
    auto addLink = [&](ConceptId source, ConceptId target, RoleId role) -> bool {
        if (contexts[source].hasLink(role, target)) return false;
        contexts[source].link_map[role].push_back(target);
        contexts[target].pred_map[role].push_back(source);
        return true;
    };
    
    while (!worklist.empty() || !link_worklist.empty()) {
        while (!worklist.empty()) {
            auto item = worklist.back();
            worklist.pop_back();
            
            ConceptId c = item.concept;
            ConceptId d = item.added;
            
            // CR1
            if (d < store.sub_to_sups.size()) {
                for (ConceptId e : store.sub_to_sups[d]) {
                    if (!contexts[c].hasSuper(e)) {
                        contexts[c].addSuper(e);
                        worklist.push_back({c, e});
                    }
                }
            }
            
            // CR2
            if (d < store.conj_index.size() && !store.conj_index[d].empty()) {
                for (const auto& [d2, results] : store.conj_index[d]) {
                    if (contexts[c].hasSuper(d2)) {
                        for (ConceptId e : results) {
                            if (!contexts[c].hasSuper(e)) {
                                contexts[c].addSuper(e);
                                worklist.push_back({c, e});
                            }
                        }
                    }
                }
            }
            
            // CR3
            if (d < store.exist_right.size()) {
                for (const auto& rf : store.exist_right[d]) {
                    if (addLink(c, rf.fill, rf.role)) {
                        link_worklist.push_back({c, rf.role, rf.fill});
                    }
                }
            }
            
            // CR4 backward
            for (RoleId r = 0; r < num_roles; r++) {
                for (ConceptId pred : contexts[c].pred_map[r]) {
                    if (r < store.exist_left.size() && store.exist_left[r].count(d)) {
                        for (ConceptId f : store.exist_left[r].at(d)) {
                            if (!contexts[pred].hasSuper(f)) {
                                contexts[pred].addSuper(f);
                                worklist.push_back({pred, f});
                            }
                        }
                    }
                }
            }
        }
        
        while (!link_worklist.empty()) {
            auto li = link_worklist.back();
            link_worklist.pop_back();
            
            ConceptId c = li.source;
            RoleId r = li.role;
            ConceptId d = li.target;
            
            // CR4 forward
            if (r < store.exist_left.size() && !store.exist_left[r].empty()) {
                for (ConceptId e : contexts[d].super_set) {
                    if (store.exist_left[r].count(e)) {
                        for (ConceptId f : store.exist_left[r].at(e)) {
                            if (!contexts[c].hasSuper(f)) {
                                contexts[c].addSuper(f);
                                worklist.push_back({c, f});
                            }
                        }
                    }
                }
            }
            
            // CR5
            if (contexts[d].hasSuper(BOTTOM)) {
                if (!contexts[c].hasSuper(BOTTOM)) {
                    contexts[c].addSuper(BOTTOM);
                    worklist.push_back({c, BOTTOM});
                }
            }
        }
    }
    
    return contexts;
}

size_t countInferred(const std::vector<Context>& contexts) {
    size_t total = 0;
    for (size_t c = 2; c < contexts.size(); c++) {
        if (contexts[c].super_set.size() > 2) {
            total += contexts[c].super_set.size() - 2;
        }
    }
    return total;
}

int main(int argc, char** argv) {
    if (argc < 2) {
        std::cerr << "Usage: " << argv[0] << " <input.obo>\n";
        return 1;
    }
    
    using Clock = std::chrono::high_resolution_clock;
    
    // Parse
    auto parse_start = Clock::now();
    
    std::unordered_map<std::string, ConceptId> concept_idx;
    std::unordered_map<std::string, RoleId> role_idx;
    concept_idx["owl:Thing"] = 0;
    concept_idx["owl:Nothing"] = 1;
    ConceptId next_concept = 2;
    RoleId next_role = 0;
    
    std::vector<std::pair<ConceptId, ConceptId>> subsumptions;
    std::vector<std::tuple<ConceptId, RoleId, ConceptId>> relations;
    
    std::ifstream file(argv[1]);
    std::string line;
    std::string current_id;
    bool in_term = false;
    bool is_obsolete = false;
    
    while (std::getline(file, line)) {
        size_t start = line.find_first_not_of(" \t");
        if (start == std::string::npos) continue;
        line = line.substr(start);
        
        // Trim trailing
        while (!line.empty() && (line.back() == '\n' || line.back() == '\r' || line.back() == ' ')) {
            line.pop_back();
        }
        
        if (line == "[Term]") {
            in_term = true;
            current_id.clear();
            is_obsolete = false;
            continue;
        }
        if (!line.empty() && line[0] == '[') {
            in_term = false;
            continue;
        }
        if (!in_term) continue;
        
        if (line.rfind("id:", 0) == 0) {
            current_id = line.substr(4);
            if (concept_idx.find(current_id) == concept_idx.end()) {
                concept_idx[current_id] = next_concept++;
            }
        } else if (line.rfind("is_obsolete:", 0) == 0) {
            is_obsolete = line.find("true") != std::string::npos;
        } else if (!is_obsolete && !current_id.empty()) {
            if (line.rfind("is_a:", 0) == 0) {
                size_t bang = line.find('!');
                std::string target = line.substr(5, bang == std::string::npos ? std::string::npos : bang - 5);
                // trim
                size_t s = target.find_first_not_of(" ");
                size_t e = target.find_last_not_of(" ");
                if (s != std::string::npos) target = target.substr(s, e - s + 1);
                
                if (concept_idx.find(target) == concept_idx.end()) {
                    concept_idx[target] = next_concept++;
                }
                subsumptions.push_back({concept_idx[current_id], concept_idx[target]});
            } else if (line.rfind("relationship:", 0) == 0) {
                std::istringstream iss(line.substr(13));
                std::string role, target;
                if (iss >> role >> target) {
                    if (role_idx.find(role) == role_idx.end()) {
                        role_idx[role] = next_role++;
                    }
                    if (concept_idx.find(target) == concept_idx.end()) {
                        concept_idx[target] = next_concept++;
                    }
                    relations.push_back({concept_idx[current_id], role_idx[role], concept_idx[target]});
                }
            }
        }
    }
    
    auto parse_end = Clock::now();
    auto parse_time = std::chrono::duration<double>(parse_end - parse_start).count();
    std::cerr << "Parsed " << next_concept << " concepts in " << parse_time << "s\n";
    
    // Build axiom store
    auto build_start = Clock::now();
    AxiomStore store(next_concept, next_role + 1);
    
    for (const auto& [sub, sup] : subsumptions) {
        store.addSubsumption(sub, sup);
    }
    for (const auto& [sub, role, target] : relations) {
        store.addExistRight(sub, role, target);
    }
    
    auto build_end = Clock::now();
    auto build_time = std::chrono::duration<double>(build_end - build_start).count();
    std::cerr << "Built axiom store in " << build_time << "s\n";
    
    // Saturate
    auto sat_start = Clock::now();
    auto contexts = saturate(store, next_concept, next_role + 1);
    auto sat_end = Clock::now();
    auto sat_time = std::chrono::duration<double>(sat_end - sat_start).count();
    std::cerr << "Saturation complete in " << sat_time << "s\n";
    
    // Count inferred
    size_t inferred = countInferred(contexts);
    
    std::cerr << "\n=== Classification Stats ===\n";
    std::cerr << "Concepts: " << (next_concept - 2) << "\n";
    std::cerr << "Inferred subsumptions: " << inferred << "\n";
    std::cerr << "Total time: " << (parse_time + build_time + sat_time) << "s\n";
    
    return 0;
}
