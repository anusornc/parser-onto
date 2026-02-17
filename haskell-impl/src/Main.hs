module Main where

import Reasoner
import qualified Data.Map.Strict as M
import qualified Data.Set as S
import Data.Text (Text)
import qualified Data.Text as T
import qualified Data.Text.IO as TIO
import Data.Time.Clock (getCurrentTime, diffUTCTime, UTCTime)
import System.Environment (getArgs)
import Control.Monad (forM_, when)
import Data.Char (isSpace)
import Data.Int (Int32)

type ConceptId = Int32
type RoleId = Int32

data ParseState = ParseState
    { psConceptIdx :: M.Map Text ConceptId
    , psRoleIdx    :: M.Map Text RoleId
    , psNextConcept :: ConceptId
    , psNextRole    :: RoleId
    , psSubsumptions :: [(ConceptId, ConceptId)]
    , psRelations   :: [(ConceptId, RoleId, ConceptId)]
    , psCurrentId   :: Maybe ConceptId
    , psInTerm      :: Bool
    , psIsObsolete  :: Bool
    }

initialState :: ParseState
initialState = ParseState
    { psConceptIdx = M.fromList [("owl:Thing", 0), ("owl:Nothing", 1)]
    , psRoleIdx = M.empty
    , psNextConcept = 2
    , psNextRole = 0
    , psSubsumptions = []
    , psRelations = []
    , psCurrentId = Nothing
    , psInTerm = False
    , psIsObsolete = False
    }

main :: IO ()
main = do
    args <- getArgs
    if null args
        then putStrLn "Usage: el-reasoner <input.obo>"
        else do
            let inputPath = head args
            
            parseStart <- getCurrentTime
            contents <- TIO.readFile inputPath
            let state = parseOBO contents
            parseEnd <- getCurrentTime
            let parseTime = diffUTCTime parseEnd parseStart
            
            putStrLn $ "Parsed " ++ show (psNextConcept state - 2) ++ " concepts in " ++ show parseTime
            
            buildStart <- getCurrentTime
            let store = buildAxiomStore state
            buildEnd <- getCurrentTime
            let buildTime = diffUTCTime buildEnd buildStart
            putStrLn $ "Built axiom store in " ++ show buildTime
            
            satStart <- getCurrentTime
            let contexts = saturate store
            satEnd <- getCurrentTime
            let satTime = diffUTCTime satEnd satStart
            putStrLn $ "Saturation complete in " ++ show satTime
            
            let inferred = countInferred contexts
            
            putStrLn "\n=== Classification Stats ==="
            putStrLn $ "Concepts: " ++ show (psNextConcept state - 2)
            putStrLn $ "Inferred subsumptions: " ++ show inferred
            putStrLn $ "Total time: " ++ show (parseTime + buildTime + satTime)

parseOBO :: Text -> ParseState
parseOBO contents = go initialState (T.lines contents)
  where
    go state [] = state
    go state (line:lines) =
        let line' = T.strip line
            state' = processLine state line'
        in go state' lines

processLine :: ParseState -> Text -> ParseState
processLine state line
    | line == "[Term]" = state { psInTerm = True, psCurrentId = Nothing, psIsObsolete = False }
    | T.head line == '[' = state { psInTerm = False }
    | not (psInTerm state) = state
    | otherwise = processTermLine state line

processTermLine :: ParseState -> Text -> ParseState
processTermLine state line
    | "id:" `T.isPrefixOf` line = 
        let ident = T.strip $ T.drop 3 line
            (cid, idx') = case M.lookup ident (psConceptIdx state) of
                Just i -> (i, psConceptIdx state)
                Nothing -> let i = psNextConcept state
                           in (i, M.insert ident i (psConceptIdx state))
            nextC = if M.member ident (psConceptIdx state) 
                    then psNextConcept state 
                    else psNextConcept state + 1
        in state { psCurrentId = Just cid, psConceptIdx = idx', psNextConcept = nextC }
    
    | "is_obsolete:" `T.isPrefixOf` line = 
        state { psIsObsolete = "true" `T.isInfixOf` line }
    
    | psIsObsolete state = state
    
    | "is_a:" `T.isPrefixOf` line, Just sub <- psCurrentId state =
        let rest = T.strip $ T.drop 5 line
            target = T.strip $ T.takeWhile (/= '!') rest
            (sup, idx') = case M.lookup target (psConceptIdx state) of
                Just i -> (i, psConceptIdx state)
                Nothing -> let i = psNextConcept state
                           in (i, M.insert target i (psConceptIdx state))
            nextC = psNextConcept state + (if M.member target (psConceptIdx state) then 0 else 1)
            subs' = (sub, sup) : psSubsumptions state
        in state { psSubsumptions = subs', psConceptIdx = idx', psNextConcept = nextC }
    
    | "relationship:" `T.isPrefixOf` line, Just sub <- psCurrentId state =
        let rest = T.strip $ T.drop 13 line
            parts = T.words rest
        in case parts of
            (role:target:_) ->
                let (rid, ridx') = case M.lookup role (psRoleIdx state) of
                        Just i -> (i, psRoleIdx state)
                        Nothing -> let i = psNextRole state
                                   in (i, M.insert role i (psRoleIdx state))
                    (tid, cidx') = case M.lookup target (psConceptIdx state) of
                        Just i -> (i, psConceptIdx state)
                        Nothing -> let i = psNextConcept state
                                   in (i, M.insert target i (psConceptIdx state))
                    rels' = (sub, rid, tid) : psRelations state
                    nextR = psNextRole state + (if M.member role (psRoleIdx state) then 0 else 1)
                    nextC = psNextConcept state + (if M.member target (psConceptIdx state) then 0 else 1)
                in state { psRelations = rels', psRoleIdx = ridx', psConceptIdx = cidx'
                         , psNextRole = nextR, psNextConcept = nextC }
            _ -> state
    
    | otherwise = state

buildAxiomStore :: ParseState -> AxiomStore
buildAxiomStore state =
    let nc = fromIntegral (psNextConcept state)
        nr = fromIntegral (psNextRole state) + 1
        store0 = newAxiomStore nc nr
        store1 = foldl (\s (sub, sup) -> addSubsumption s sub sup) store0 (psSubsumptions state)
        store2 = foldl (\s (sub, role, target) -> addExistRight s sub role target) store1 (psRelations state)
    in store2
