// Copyright 2021-2025 Zenauth Ltd.
// SPDX-License-Identifier: Apache-2.0

package engine

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"reflect"
	"sort"

	celast "github.com/google/cel-go/common/ast"
	"github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	"go.uber.org/multierr"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/types/known/structpb"

	auditv1 "github.com/cerbos/cerbos/api/genpb/cerbos/audit/v1"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	enginev1 "github.com/cerbos/cerbos/api/genpb/cerbos/engine/v1"
	policyv1 "github.com/cerbos/cerbos/api/genpb/cerbos/policy/v1"
	runtimev1 "github.com/cerbos/cerbos/api/genpb/cerbos/runtime/v1"
	schemav1 "github.com/cerbos/cerbos/api/genpb/cerbos/schema/v1"
	"github.com/cerbos/cerbos/internal/conditions"
	"github.com/cerbos/cerbos/internal/engine/internal"
	"github.com/cerbos/cerbos/internal/engine/ruletable"
	"github.com/cerbos/cerbos/internal/engine/tracer"
	"github.com/cerbos/cerbos/internal/namer"
	"github.com/cerbos/cerbos/internal/policy"
	"github.com/cerbos/cerbos/internal/schema"
)

const (
	noMatchScopePermissions = "NO_MATCH_FOR_SCOPE_PERMISSIONS"
	conditionNotSatisfied   = "Condition not satisfied"
)

var ErrPolicyNotExecutable = errors.New("policy not executable")

type evalParams struct {
	globals              map[string]any
	nowFunc              conditions.NowFunc
	defaultPolicyVersion string
	lenientScopeSearch   bool
}

func defaultEvalParams(conf *Conf) evalParams {
	return evalParams{
		globals:              conf.Globals,
		defaultPolicyVersion: conf.DefaultPolicyVersion,
		lenientScopeSearch:   conf.LenientScopeSearch,
	}
}

type evalContext struct {
	request               *enginev1.Request
	runtime               *enginev1.Runtime
	effectiveDerivedRoles internal.StringSet
	evalParams
}

func newEvalContext(ep evalParams, request *enginev1.Request) *evalContext {
	return &evalContext{
		evalParams: ep,
		request:    request,
	}
}

func (ec *evalContext) withEffectiveDerivedRoles(effectiveDerivedRoles internal.StringSet) *evalContext {
	return &evalContext{
		evalParams:            ec.evalParams,
		request:               ec.request,
		effectiveDerivedRoles: effectiveDerivedRoles,
	}
}

func (ec *evalContext) lazyRuntime() any { // We have to return `any` rather than `*enginev1.Runtime` here to be able to use this function as a lazy binding in the CEL evaluator.
	if ec.runtime == nil {
		ec.runtime = &enginev1.Runtime{}
		if len(ec.effectiveDerivedRoles) > 0 {
			ec.runtime.EffectiveDerivedRoles = ec.effectiveDerivedRoles.Values()
			sort.Strings(ec.runtime.EffectiveDerivedRoles)
		}
	}

	return ec.runtime
}

type Evaluator interface {
	Evaluate(context.Context, tracer.Context, *enginev1.CheckInput) (*PolicyEvalResult, error)
}

func NewRuleTableEvaluator(rt *ruletable.RuleTable, schemaMgr schema.Manager, eparams evalParams) Evaluator {
	return &ruleTableEvaluator{
		RuleTable:  rt,
		schemaMgr:  schemaMgr,
		evalParams: eparams,
	}
}

type ruleTableEvaluator struct {
	*ruletable.RuleTable
	schemaMgr  schema.Manager
	evalParams evalParams
}

func (rte *ruleTableEvaluator) Evaluate(ctx context.Context, tctx tracer.Context, input *enginev1.CheckInput) (*PolicyEvalResult, error) {
	principalVersion := input.Principal.PolicyVersion
	if principalVersion == "" {
		principalVersion = rte.evalParams.defaultPolicyVersion
	}

	resourceVersion := input.Resource.PolicyVersion
	if resourceVersion == "" {
		resourceVersion = rte.evalParams.defaultPolicyVersion
	}

	trail := newAuditTrail(make(map[string]*policyv1.SourceAttributes))
	result := newEvalResult(input.Actions, trail)

	if !rte.evalParams.lenientScopeSearch &&
		!rte.ScopeExists(policy.PrincipalKind, input.Principal.Scope) &&
		!rte.ScopeExists(policy.ResourceKind, input.Resource.Scope) {
		return result, nil
	}

	principalScopes, principalPolicyKey, _ := rte.GetAllScopes(policy.PrincipalKind, input.Principal.Scope, input.Principal.Id, principalVersion)
	resourceScopes, resourcePolicyKey, fqn := rte.GetAllScopes(policy.ResourceKind, input.Resource.Scope, input.Resource.Kind, resourceVersion)

	pctx := tctx.StartPolicy(fqn)

	// validate the input
	vr, err := rte.schemaMgr.ValidateCheckInput(ctx, rte.GetSchema(fqn), input)
	if err != nil {
		pctx.Failed(err, "Error during validation")

		return nil, fmt.Errorf("failed to validate input: %w", err)
	}

	if len(vr.Errors) > 0 {
		result.ValidationErrors = vr.Errors.SchemaErrors()

		pctx.Failed(vr.Errors, "Validation errors")

		if vr.Reject {
			for _, action := range input.Actions {
				actx := pctx.StartAction(action)

				result.setEffect(action, EffectInfo{Effect: effectv1.Effect_EFFECT_DENY, Policy: resourcePolicyKey})

				actx.AppliedEffect(effectv1.Effect_EFFECT_DENY, "Rejected due to validation failures")
			}
			return result, nil
		}
	}

	request := checkInputToRequest(input)
	evalCtx := newEvalContext(rte.evalParams, request)

	actionsToResolve := result.unresolvedActions()
	if len(actionsToResolve) == 0 {
		return result, nil
	}

	sanitizedResource := namer.SanitizedResource(input.Resource.Kind)
	scopedPrincipalExists := rte.ScopedPrincipalExists(principalVersion, principalScopes)
	scopedResourceExists := rte.ScopedResourceExists(resourceVersion, sanitizedResource, resourceScopes)
	if !scopedPrincipalExists && !scopedResourceExists {
		return result, nil
	}

	allRoles := rte.GetParentRoles(input.Resource.Scope, input.Principal.Roles)
	includingParentRoles := make(map[string]struct{})
	for _, r := range allRoles {
		includingParentRoles[r] = struct{}{}
	}

	scopes := rte.CombineScopes(principalScopes, resourceScopes)
	candidateRows := rte.GetRows(resourceVersion, sanitizedResource, scopes, allRoles, actionsToResolve)

	varCache := make(map[string]map[string]any)
	// We can cache evaluated conditions for combinations of parameters and conditions.
	// We use a compound key comprising the parameter origin and the rule FQN.
	conditionCache := make(map[string]bool)

	processedScopedDerivedRoles := make(map[string]struct{})
	policyTypes := []policy.Kind{policy.PrincipalKind, policy.ResourceKind}
	for _, action := range actionsToResolve {
		actx := pctx.StartAction(action)

		var actionEffectInfo EffectInfo
		var mainPolicyKey string
		for _, pt := range policyTypes {
			if pt == policy.PrincipalKind {
				mainPolicyKey = principalPolicyKey
			} else {
				mainPolicyKey = resourcePolicyKey
			}

			// Reset `actionEffectInfo` for this policy type with the correct policy key.
			// This ensures we use the right policy name if no rules match
			actionEffectInfo.Effect = effectv1.Effect_EFFECT_NO_MATCH

			for i, role := range input.Principal.Roles {
				// Principal rules are role agnostic (they treat the rows as having a `*` role). Therefore we can
				// break out of the loop after the first iteration as it covers all potential principal rows.
				if i > 0 && pt == policy.PrincipalKind {
					break
				}

				roleEffectSet := make(map[effectv1.Effect]struct{})
				roleEffectInfo := EffectInfo{
					Effect: effectv1.Effect_EFFECT_NO_MATCH,
					Policy: noPolicyMatch,
				}

				// a "policy" exists, regardless of potentially matching rules, so we update the policyKey
				if pt == policy.ResourceKind && scopedResourceExists ||
					pt == policy.PrincipalKind && scopedPrincipalExists {
					roleEffectInfo.Policy = mainPolicyKey
				}

				parentRoles := rte.GetParentRoles(input.Resource.Scope, []string{role})

			scopesLoop:
				for _, scope := range scopes {
					sctx := actx.StartScope(scope)

					// This is for backwards compatibility with effectiveDerivedRoles.
					// If we reach this point, we can assert that the given {origin policy + scope} combination has been evaluated
					// and therefore we build the effectiveDerivedRoles from those referenced in the policy.
					if pt == policy.ResourceKind { //nolint:nestif
						if _, ok := processedScopedDerivedRoles[scope]; !ok { //nolint:nestif
							effectiveDerivedRoles := make(internal.StringSet)
							if drs := rte.GetDerivedRoles(namer.ResourcePolicyFQN(input.Resource.Kind, resourceVersion, scope)); drs != nil {
								for name, dr := range drs {
									drctx := tctx.StartPolicy(dr.OriginFqn).StartDerivedRole(name)
									if !internal.SetIntersects(dr.ParentRoles, includingParentRoles) {
										drctx.Skipped(nil, "No matching roles")
										continue
									}

									var variables map[string]any
									key := namer.DerivedRolesFQN(name)
									if c, ok := varCache[key]; ok {
										variables = c
									} else {
										var err error
										variables, err = evalCtx.evaluateVariables(ctx, drctx.StartVariables(), dr.Constants, dr.OrderedVariables)
										if err != nil {
											return nil, err
										}
										varCache[key] = variables
									}

									// we don't use `conditionCache` as we don't do any evaluations scoped solely to derived role conditions
									ok, err := evalCtx.satisfiesCondition(ctx, drctx.StartCondition(), dr.Condition, dr.Constants, variables)
									if err != nil {
										continue
									}

									if ok {
										effectiveDerivedRoles[name] = struct{}{}
										result.EffectiveDerivedRoles[name] = struct{}{}
									}
								}
							}

							evalCtx = evalCtx.withEffectiveDerivedRoles(effectiveDerivedRoles)

							processedScopedDerivedRoles[scope] = struct{}{}
						}
					}

					if roleEffectInfo.Effect != effectv1.Effect_EFFECT_NO_MATCH {
						break
					}

					// Only process rows that match the current policy type
					for _, row := range candidateRows {
						if !row.Matches(pt, scope, action, input.Principal.Id, parentRoles) {
							continue
						}

						rulectx := sctx.StartRule(row.Name)

						if m := rte.GetMeta(row.OriginFqn); m != nil && m.GetSourceAttributes() != nil {
							maps.Copy(result.AuditTrail.EffectivePolicies, m.GetSourceAttributes())
						}

						var constants map[string]any
						var variables map[string]any
						if row.Params != nil {
							constants = row.Params.Constants
							if c, ok := varCache[row.Params.Key]; ok {
								variables = c
							} else {
								var err error
								variables, err = evalCtx.evaluateCELProgramsOrVariables(ctx, pctx, constants, row.Params.CelPrograms, row.Params.Variables)
								if err != nil {
									pctx.Skipped(err, "Error evaluating variables")
									return nil, err
								}
								varCache[row.Params.Key] = variables
							}
						}

						var satisfiesCondition bool
						if c, ok := conditionCache[row.EvaluationKey]; ok { //nolint:nestif
							satisfiesCondition = c
						} else {
							// We evaluate the derived role condition (if any) first, as this leads to a more sane engine trace output.
							if row.DerivedRoleCondition != nil {
								drctx := rulectx.StartDerivedRole(row.OriginDerivedRole)
								var derivedRoleConstants map[string]any
								var derivedRoleVariables map[string]any
								if row.DerivedRoleParams != nil {
									derivedRoleConstants = row.DerivedRoleParams.Constants
									if c, ok := varCache[row.DerivedRoleParams.Key]; ok {
										derivedRoleVariables = c
									} else {
										var err error
										derivedRoleVariables, err = evalCtx.evaluateCELProgramsOrVariables(ctx, drctx, derivedRoleConstants, row.DerivedRoleParams.CelPrograms, row.DerivedRoleParams.Variables)
										if err != nil {
											drctx.Skipped(err, "Error evaluating derived role variables")
											return nil, err
										}
										varCache[row.DerivedRoleParams.Key] = derivedRoleVariables
									}
								}

								// Derived role engine trace logs are handled above. Because derived role conditions are baked into the rule table rows, we don't want to
								// confuse matters by adding condition trace logs if a rule is referencing a derived role, so we pass a no-op context here.
								// TODO(saml) we could probably pre-compile the condition also
								drSatisfied, err := evalCtx.satisfiesCondition(ctx, tracer.Start(nil), row.DerivedRoleCondition, derivedRoleConstants, derivedRoleVariables)
								if err != nil {
									rulectx.Skipped(err, "Error evaluating derived role condition")
									continue
								}

								// terminate early if the derived role condition isn't satisfied, which is consistent with the pre-rule table implementation
								if !drSatisfied {
									rulectx.Skipped(err, "No matching derived roles")
									conditionCache[row.EvaluationKey] = false
									continue
								}
							}

							isSatisfied, err := evalCtx.satisfiesCondition(ctx, rulectx.StartCondition(), row.Condition, constants, variables)
							if err != nil {
								rulectx.Skipped(err, "Error evaluating condition")
								continue
							}

							conditionCache[row.EvaluationKey] = isSatisfied
							satisfiesCondition = isSatisfied
						}

						if satisfiesCondition { //nolint:nestif
							var outputExpr *exprpb.CheckedExpr
							if row.EmitOutput != nil && row.EmitOutput.When != nil && row.EmitOutput.When.RuleActivated != nil {
								outputExpr = row.EmitOutput.When.RuleActivated.Checked
							}

							if outputExpr != nil {
								octx := rulectx.StartOutput(row.Name)
								output := &enginev1.OutputEntry{
									Src: namer.RuleFQN(rte.GetMeta(row.OriginFqn), row.Scope, row.Name),
									Val: evalCtx.evaluateProtobufValueCELExpr(ctx, outputExpr, row.Params.Constants, variables),
								}
								result.Outputs = append(result.Outputs, output)
								octx.ComputedOutput(output)
							}

							roleEffectSet[row.Effect] = struct{}{}
							if row.Effect == effectv1.Effect_EFFECT_DENY {
								roleEffectInfo.Effect = effectv1.Effect_EFFECT_DENY
								roleEffectInfo.Scope = scope
								if row.FromRolePolicy {
									// Implicit DENY generated as a result of no matching role policy action
									// needs to be attributed to said role policy
									roleEffectInfo.Policy = namer.PolicyKeyFromFQN(row.OriginFqn)
								}
								break scopesLoop
							} else if row.NoMatchForScopePermissions {
								roleEffectInfo.Policy = noMatchScopePermissions
								roleEffectInfo.Scope = scope
							}
						} else {
							if row.EmitOutput != nil && row.EmitOutput.When != nil && row.EmitOutput.When.ConditionNotMet != nil {
								octx := rulectx.StartOutput(row.Name)
								output := &enginev1.OutputEntry{
									Src: namer.RuleFQN(rte.GetMeta(row.OriginFqn), row.Scope, row.Name),
									Val: evalCtx.evaluateProtobufValueCELExpr(ctx, row.EmitOutput.When.ConditionNotMet.Checked, row.Params.Constants, variables),
								}
								result.Outputs = append(result.Outputs, output)
								octx.ComputedOutput(output)
							}
							rulectx.Skipped(nil, conditionNotSatisfied)
						}
					}

					if _, hasAllow := roleEffectSet[effectv1.Effect_EFFECT_ALLOW]; hasAllow {
						switch rte.GetScopeScopePermissions(scope) { //nolint:exhaustive
						case policyv1.ScopePermissions_SCOPE_PERMISSIONS_REQUIRE_PARENTAL_CONSENT_FOR_ALLOWS:
							delete(roleEffectSet, effectv1.Effect_EFFECT_ALLOW)
						case policyv1.ScopePermissions_SCOPE_PERMISSIONS_OVERRIDE_PARENT:
							roleEffectInfo.Effect = effectv1.Effect_EFFECT_ALLOW
							roleEffectInfo.Scope = scope
							break scopesLoop
						}
					}
				}

				// Match the first result
				if actionEffectInfo.Effect == effectv1.Effect_EFFECT_NO_MATCH {
					actionEffectInfo = roleEffectInfo
				}

				if roleEffectInfo.Effect == effectv1.Effect_EFFECT_ALLOW {
					// Finalise and return the first independent ALLOW
					actionEffectInfo = roleEffectInfo
					break
				} else if roleEffectInfo.Effect == effectv1.Effect_EFFECT_DENY &&
					actionEffectInfo.Policy == noMatchScopePermissions &&
					roleEffectInfo.Policy != noMatchScopePermissions {
					// Override `noMatchScopePermissions` DENYs with explicit ones for clarity
					actionEffectInfo = roleEffectInfo
				}
			}

			// Skip to next action if this action already has a definitive result from principal policies
			if actionEffectInfo.Effect == effectv1.Effect_EFFECT_ALLOW || actionEffectInfo.Effect == effectv1.Effect_EFFECT_DENY {
				break
			}
		}

		if actionEffectInfo.Effect == effectv1.Effect_EFFECT_NO_MATCH {
			actionEffectInfo.Effect = effectv1.Effect_EFFECT_DENY
		}

		result.setEffect(action, actionEffectInfo)
		actx.AppliedEffect(actionEffectInfo.Effect, "")
	}

	return result, nil
}

func (ec *evalContext) evaluateCELProgramsOrVariables(ctx context.Context, tctx tracer.Context, constants map[string]any, celPrograms []*ruletable.CelProgram, variables []*runtimev1.Variable) (map[string]any, error) {
	// if nowFunc is provided, we need to recompute the cel.Program to handle the custom time decorator, otherwise we can reuse the precomputed program
	// from build-time.
	if ec.nowFunc == nil {
		return ec.evaluatePrograms(constants, celPrograms)
	}

	return ec.evaluateVariables(ctx, tctx.StartVariables(), constants, variables)
}

func (ec *evalContext) evaluateVariables(ctx context.Context, tctx tracer.Context, constants map[string]any, variables []*runtimev1.Variable) (map[string]any, error) {
	var errs error
	evalVars := make(map[string]any, len(variables))
	for _, variable := range variables {
		vctx := tctx.StartVariable(variable.Name, variable.Expr.Original)
		val, err := ec.evaluateCELExprToRaw(ctx, variable.Expr.Checked, constants, evalVars)
		if err != nil {
			vctx.Skipped(err, "Failed to evaluate expression")
			errs = multierr.Append(errs, fmt.Errorf("error evaluating `%s := %s`: %w", variable.Name, variable.Expr.Original, err))
			continue
		}

		evalVars[variable.Name] = val
		vctx.ComputedResult(val)
	}

	return evalVars, errs
}

func (ec *evalContext) buildEvalVars(constants, variables map[string]any) map[string]any {
	return map[string]any{
		conditions.CELRequestIdent:    ec.request,
		conditions.CELResourceAbbrev:  ec.request.Resource,
		conditions.CELPrincipalAbbrev: ec.request.Principal,
		conditions.CELRuntimeIdent:    ec.lazyRuntime,
		conditions.CELConstantsIdent:  constants,
		conditions.CELConstantsAbbrev: constants,
		conditions.CELVariablesIdent:  variables,
		conditions.CELVariablesAbbrev: variables,
		conditions.CELGlobalsIdent:    ec.globals,
		conditions.CELGlobalsAbbrev:   ec.globals,
	}
}

func (ec *evalContext) evaluatePrograms(constants map[string]any, celPrograms []*ruletable.CelProgram) (map[string]any, error) {
	var errs error

	evalVars := make(map[string]any, len(celPrograms))
	for _, prg := range celPrograms {
		result, _, err := prg.Prog.Eval(ec.buildEvalVars(constants, evalVars))
		if err != nil {
			errs = multierr.Append(errs, fmt.Errorf("error evaluating `%s`: %w", prg.Name, err))
			continue
		}

		evalVars[prg.Name] = result.Value()
	}

	return evalVars, errs
}

func (ec *evalContext) satisfiesCondition(ctx context.Context, tctx tracer.Context, cond *runtimev1.Condition, constants, variables map[string]any) (bool, error) {
	if cond == nil {
		tctx.ComputedBoolResult(true, nil, "")
		return true, nil
	}

	switch t := cond.Op.(type) {
	case *runtimev1.Condition_Expr:
		ectx := tctx.StartExpr(t.Expr.Original)
		val, err := ec.evaluateBoolCELExpr(ctx, t.Expr.Checked, constants, variables)
		if err != nil {
			ectx.ComputedBoolResult(false, err, "Failed to evaluate expression")
			return false, fmt.Errorf("failed to evaluate `%s`: %w", t.Expr.Original, err)
		}

		ectx.ComputedBoolResult(val, nil, "")
		return val, nil

	case *runtimev1.Condition_All:
		actx := tctx.StartConditionAll()
		for i, expr := range t.All.Expr {
			val, err := ec.satisfiesCondition(ctx, actx.StartNthCondition(i), expr, constants, variables)
			if err != nil {
				actx.ComputedBoolResult(false, err, "Short-circuited")
				return false, err
			}

			if !val {
				actx.ComputedBoolResult(false, nil, "Short-circuited")
				return false, nil
			}
		}

		actx.ComputedBoolResult(true, nil, "")
		return true, nil

	case *runtimev1.Condition_Any:
		actx := tctx.StartConditionAny()
		for i, expr := range t.Any.Expr {
			val, err := ec.satisfiesCondition(ctx, actx.StartNthCondition(i), expr, constants, variables)
			if err != nil {
				actx.ComputedBoolResult(false, err, "Short-circuited")
				return false, err
			}

			if val {
				actx.ComputedBoolResult(true, nil, "Short-circuited")
				return true, nil
			}
		}

		actx.ComputedBoolResult(false, nil, "")
		return false, nil

	case *runtimev1.Condition_None:
		actx := tctx.StartConditionNone()
		for i, expr := range t.None.Expr {
			val, err := ec.satisfiesCondition(ctx, actx.StartNthCondition(i), expr, constants, variables)
			if err != nil {
				actx.ComputedBoolResult(false, err, "Short-circuited")
				return false, err
			}

			if val {
				actx.ComputedBoolResult(false, nil, "Short-circuited")
				return false, nil
			}
		}

		actx.ComputedBoolResult(true, nil, "")
		return true, nil

	default:
		err := fmt.Errorf("unknown op type %T", t)
		tctx.ComputedBoolResult(false, err, "Unknown op type")
		return false, err
	}
}

func (ec *evalContext) evaluateBoolCELExpr(ctx context.Context, expr *exprpb.CheckedExpr, constants, variables map[string]any) (bool, error) {
	val, err := ec.evaluateCELExprToRaw(ctx, expr, constants, variables)
	if err != nil {
		return false, err
	}

	if val == nil {
		return false, nil
	}

	boolVal, ok := val.(bool)
	if !ok {
		return false, nil
	}

	return boolVal, nil
}

func (ec *evalContext) evaluateProtobufValueCELExpr(ctx context.Context, expr *exprpb.CheckedExpr, constants, variables map[string]any) *structpb.Value {
	result, err := ec.evaluateCELExpr(ctx, expr, constants, variables)
	if err != nil {
		return structpb.NewStringValue("<failed to evaluate expression>")
	}

	if result == nil {
		return nil
	}

	val, err := result.ConvertToNative(reflect.TypeOf(&structpb.Value{}))
	if err != nil {
		return structpb.NewStringValue("<failed to convert evaluation to protobuf value>")
	}

	pbVal, ok := val.(*structpb.Value)
	if !ok {
		// Something is broken in `ConvertToNative`
		return structpb.NewStringValue("<failed to convert evaluation to protobuf value>")
	}

	return pbVal
}

func (ec *evalContext) evaluateCELExpr(ctx context.Context, expr *exprpb.CheckedExpr, constants, variables map[string]any) (ref.Val, error) {
	if expr == nil {
		return nil, nil
	}

	ast, err := celast.ToAST(expr)
	if err != nil {
		return nil, err
	}
	result, _, err := conditions.ContextEval(ctx, conditions.StdEnv, ast, ec.buildEvalVars(constants, variables), ec.nowFunc)
	if err != nil {
		// ignore expressions that are invalid
		if types.IsError(result) {
			return nil, nil
		}

		return nil, err
	}

	return result, nil
}

func (ec *evalContext) evaluateCELExprToRaw(ctx context.Context, expr *exprpb.CheckedExpr, constants, variables map[string]any) (any, error) {
	result, err := ec.evaluateCELExpr(ctx, expr, constants, variables)
	if err != nil {
		return nil, err
	}

	if result == nil {
		return nil, nil
	}

	return result.Value(), nil
}

type EffectInfo struct {
	Policy string
	Scope  string
	Effect effectv1.Effect
}

type PolicyEvalResult struct {
	Effects               map[string]EffectInfo
	EffectiveDerivedRoles map[string]struct{}
	toResolve             map[string]struct{}
	AuditTrail            *auditv1.AuditTrail
	ValidationErrors      []*schemav1.ValidationError
	Outputs               []*enginev1.OutputEntry
}

func newEvalResult(actions []string, auditTrail *auditv1.AuditTrail) *PolicyEvalResult {
	per := &PolicyEvalResult{
		Effects:               make(map[string]EffectInfo, len(actions)),
		EffectiveDerivedRoles: make(map[string]struct{}),
		toResolve:             make(map[string]struct{}, len(actions)),
		Outputs:               []*enginev1.OutputEntry{},
		AuditTrail:            auditTrail,
	}

	for _, a := range actions {
		per.toResolve[a] = struct{}{}
	}

	return per
}

func (er *PolicyEvalResult) unresolvedActions() []string {
	if len(er.toResolve) == 0 {
		return nil
	}

	res := make([]string, len(er.toResolve))
	i := 0
	for ua := range er.toResolve {
		res[i] = ua
		i++
	}

	return res
}

// setEffect sets the effect for an action. DENY always takes precedence.
func (er *PolicyEvalResult) setEffect(action string, effect EffectInfo) {
	delete(er.toResolve, action)

	if effect.Effect == effectv1.Effect_EFFECT_DENY {
		er.Effects[action] = effect
		return
	}

	current, ok := er.Effects[action]
	if !ok {
		er.Effects[action] = effect
		return
	}

	if current.Effect != effectv1.Effect_EFFECT_DENY {
		er.Effects[action] = effect
	}
}

func checkInputToRequest(input *enginev1.CheckInput) *enginev1.Request {
	return &enginev1.Request{
		Principal: &enginev1.Request_Principal{
			Id:            input.Principal.Id,
			Roles:         input.Principal.Roles,
			Attr:          input.Principal.Attr,
			PolicyVersion: input.Principal.PolicyVersion,
			Scope:         input.Principal.Scope,
		},
		Resource: &enginev1.Request_Resource{
			Kind:          input.Resource.Kind,
			Id:            input.Resource.Id,
			Attr:          input.Resource.Attr,
			PolicyVersion: input.Resource.PolicyVersion,
			Scope:         input.Resource.Scope,
		},
		AuxData: input.AuxData,
	}
}

func newAuditTrail(srcAttr map[string]*policyv1.SourceAttributes) *auditv1.AuditTrail {
	return &auditv1.AuditTrail{EffectivePolicies: maps.Clone(srcAttr)}
}
