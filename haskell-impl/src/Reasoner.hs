{-# LANGUAGE BangPatterns #-}
{-# LANGUAGE StrictData #-}
module Reasoner where

import Data.Int (Int32)
import Data.Vector (Vector, (!))
import qualified Data.Vector as V
import qualified Data.Vector.Mutable as VM
import Data.Set (Set)
import qualified Data.Set as S
import Data.Map.Strict (Map)
import qualified Data.Map.Strict as M
import Data.IntSet (IntSet)
import qualified Data.IntSet as IS
import Data.IntMap.Strict (IntMap)
import qualified Data.IntMap.Strict as IM
import Control.Monad (forM_, when)
import Control.Monad.ST (ST, runST)
import Data.STRef

type ConceptId = Int32
type RoleId = Int32

top :: ConceptId
top = 0

bottom :: ConceptId
bottom = 1

data RoleFiller = RoleFiller
    { rfRole :: !RoleId
    , rfFill :: !ConceptId
    }

data AxiomStore = AxiomStore
    { asSubToSups   :: !(Vector [ConceptId])
    , asConjIndex   :: !(Vector (IntMap [ConceptId]))
    , asExistRight  :: !(Vector [RoleFiller])
    , asExistLeft   :: !(Vector (IntMap [ConceptId]))
    , asNumConcepts :: !Int
    , asNumRoles    :: !Int
    }

newAxiomStore :: Int -> Int -> AxiomStore
newAxiomStore numConcepts numRoles = AxiomStore
    { asSubToSups   = V.replicate numConcepts []
    , asConjIndex   = V.replicate numConcepts IM.empty
    , asExistRight  = V.replicate numConcepts []
    , asExistLeft   = V.replicate numRoles IM.empty
    , asNumConcepts = numConcepts
    , asNumRoles    = numRoles
    }

addSubsumption :: AxiomStore -> ConceptId -> ConceptId -> AxiomStore
addSubsumption store sub sup = store
    { asSubToSups = asSubToSups store V.// [(fromIntegral sub, sup : asSubToSups store V.! fromIntegral sub)]
    }

addExistRight :: AxiomStore -> ConceptId -> RoleId -> ConceptId -> AxiomStore
addExistRight store sub role fill = store
    { asExistRight = asExistRight store V.// [(fromIntegral sub, RoleFiller role fill : asExistRight store V.! fromIntegral sub)]
    }

data Context = Context
    { ctxId       :: !ConceptId
    , ctxSuperSet :: !IntSet
    , ctxLinkMap  :: !(Vector [ConceptId])
    , ctxPredMap  :: !(Vector [ConceptId])
    }

data WorkItem = WorkItem !ConceptId !ConceptId
data LinkItem = LinkItem !ConceptId !RoleId !ConceptId

saturate :: AxiomStore -> [Context]
saturate store = runST $ do
    let nc = asNumConcepts store
        nr = asNumRoles store
    
    -- Initialize contexts
    contexts <- VM.new nc
    forM_ [0..nc-1] $ \c -> do
        let ctx = Context
                { ctxId = fromIntegral c
                , ctxSuperSet = IS.fromList [fromIntegral c, fromIntegral top]
                , ctxLinkMap = V.replicate nr []
                , ctxPredMap = V.replicate nr []
                }
        VM.write contexts c ctx
    
    -- Worklists
    worklistRef <- newSTRef ([WorkItem (fromIntegral c) (fromIntegral c) | c <- [0..nc-1]] 
                          ++ [WorkItem (fromIntegral c) top | c <- [0..nc-1]])
    linkWorklistRef <- newSTRef []
    
    let processWorkItem (WorkItem c d) = do
            -- CR1
            let subs = asSubToSups store V.! fromIntegral d
            forM_ subs $ \e -> do
                ctx <- VM.read contexts (fromIntegral c)
                when (IS.notMember (fromIntegral e) (ctxSuperSet ctx)) $ do
                    let ctx' = ctx { ctxSuperSet = IS.insert (fromIntegral e) (ctxSuperSet ctx) }
                    VM.write contexts (fromIntegral c) ctx'
                    modifySTRef' worklistRef (WorkItem c e :)
            
            -- CR3
            let exr = asExistRight store V.! fromIntegral d
            forM_ exr $ \(RoleFiller r b) -> do
                added <- addLinkST contexts c b r nr
                when added $ modifySTRef' linkWorklistRef (LinkItem c r b :)
            
            -- CR4 backward
            forM_ [0..nr-1] $ \r -> do
                ctx <- VM.read contexts (fromIntegral c)
                forM_ (ctxPredMap ctx V.! r) $ \pred -> do
                    let exl = asExistLeft store V.! r
                    case IM.lookup (fromIntegral d) exl of
                        Nothing -> pure ()
                        Just sups -> forM_ sups $ \f -> do
                            predCtx <- VM.read contexts (fromIntegral pred)
                            when (IS.notMember (fromIntegral f) (ctxSuperSet predCtx)) $ do
                                let predCtx' = predCtx { ctxSuperSet = IS.insert (fromIntegral f) (ctxSuperSet predCtx) }
                                VM.write contexts (fromIntegral pred) predCtx'
                                modifySTRef' worklistRef (WorkItem pred f :)
    
    let processLinkItem (LinkItem c r d) = do
            -- CR4 forward
            let exl = asExistLeft store V.! fromIntegral r
            dCtx <- VM.read contexts (fromIntegral d)
            forM_ (IS.toList (ctxSuperSet dCtx)) $ \e -> do
                case IM.lookup e exl of
                    Nothing -> pure ()
                    Just sups -> forM_ sups $ \f -> do
                        cCtx <- VM.read contexts (fromIntegral c)
                        when (IS.notMember (fromIntegral f) (ctxSuperSet cCtx)) $ do
                            let cCtx' = cCtx { ctxSuperSet = IS.insert (fromIntegral f) (ctxSuperSet cCtx) }
                            VM.write contexts (fromIntegral c) cCtx'
                            modifySTRef' worklistRef (WorkItem c f :)
            
            -- CR5
            dCtx <- VM.read contexts (fromIntegral d)
            when (IS.member (fromIntegral bottom) (ctxSuperSet dCtx)) $ do
                cCtx <- VM.read contexts (fromIntegral c)
                when (IS.notMember (fromIntegral bottom) (ctxSuperSet cCtx)) $ do
                    let cCtx' = cCtx { ctxSuperSet = IS.insert (fromIntegral bottom) (ctxSuperSet cCtx) }
                    VM.write contexts (fromIntegral c) cCtx'
                    modifySTRef' worklistRef (WorkItem c bottom :)
    
    let loop = do
            wl <- readSTRef worklistRef
            ll <- readSTRef linkWorklistRef
            if null wl && null ll
                then pure ()
                else do
                    -- Process worklist
                    case wl of
                        (item:rest) -> do
                            writeSTRef worklistRef rest
                            processWorkItem item
                        [] -> pure ()
                    
                    -- Process link worklist if worklist empty
                    wl2 <- readSTRef worklistRef
                    when (null wl2) $ case ll of
                        (item:rest) -> do
                            writeSTRef linkWorklistRef rest
                            processLinkItem item
                        [] -> pure ()
                    
                    loop
    
    loop
    
    V.freeze contexts

addLinkST :: VM.MVector s Context -> ConceptId -> ConceptId -> RoleId -> Int -> ST s Bool
addLinkST contexts source target role numRoles = do
    srcCtx <- VM.read contexts (fromIntegral source)
    let links = ctxLinkMap srcCtx V.! fromIntegral role
    if target `elem` links
        then pure False
        else do
            let linkMap' = ctxLinkMap srcCtx V.// [(fromIntegral role, target : links)]
            VM.write contexts (fromIntegral source) (srcCtx { ctxLinkMap = linkMap' })
            
            tgtCtx <- VM.read contexts (fromIntegral target)
            let preds = ctxPredMap tgtCtx V.! fromIntegral role
            let predMap' = ctxPredMap tgtCtx V.// [(fromIntegral role, source : preds)]
            VM.write contexts (fromIntegral target) (tgtCtx { ctxPredMap = predMap' })
            
            pure True

countInferred :: [Context] -> Int
countInferred contexts = sum
    [ max 0 (IS.size (ctxSuperSet ctx) - 2)
    | ctx <- drop 2 contexts
    ]
