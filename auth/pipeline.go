package auth

import (
	"context"

	"github.com/Tsukikage7/servex/v2/observability/logger"
)

func authenticateAndAuthorize(ctx context.Context, request any, o *options, target Target) (context.Context, *Principal, error) {
	log := logger.WithComponent(logger.FromContextOr(ctx, o.logger), "Auth")

	creds, err := extractCredentials(ctx, request, o)
	if err != nil {
		if o.logger != nil {
			log.Debug("凭据提取失败", logger.Err(err))
		}
		return ctx, nil, ErrCredentialsNotFound
	}

	authCtx := WithCredentials(ctx, creds)

	principal, err := o.authenticator.Authenticate(authCtx, *creds)
	if err != nil {
		if o.logger != nil {
			log.Warn("认证失败", logger.Err(err))
		}
		return ctx, nil, err
	}
	if principal == nil {
		return ctx, nil, ErrInvalidCredentials
	}
	if principal.IsExpired() {
		if o.logger != nil {
			log.Warn("主体已过期",
				logger.String("principal_id", principal.ID),
			)
		}
		return ctx, nil, ErrCredentialsExpired
	}

	authCtx = WithPrincipal(authCtx, principal)
	target = targetWithDefaults(o.target, target)
	target = applyMethodPolicy(target, o.policyProvider)

	if err := authorizeTarget(authCtx, principal, target, o); err != nil {
		if o.logger != nil {
			log.Warn("授权失败",
				logger.String("principal_id", principal.ID),
				logger.String("resource", target.Resource),
				logger.String("action", target.Action),
				logger.Err(err),
			)
		}
		return authCtx, principal, err
	}

	return authCtx, principal, nil
}

func authorizeTarget(ctx context.Context, principal *Principal, target Target, o *options) error {
	if len(target.Permissions) == 0 {
		if o.authorizer == nil {
			return nil
		}
		return o.authorizer.Authorize(ctx, principal, target)
	}
	if o.authorizer == nil {
		return ErrForbidden
	}
	if target.AllPermissions {
		for _, permission := range target.Permissions {
			if err := o.authorizer.Authorize(ctx, principal, targetForPermission(target, permission)); err != nil {
				return err
			}
		}
		return nil
	}

	var lastErr error
	for _, permission := range target.Permissions {
		if err := o.authorizer.Authorize(ctx, principal, targetForPermission(target, permission)); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	if lastErr != nil {
		return lastErr
	}
	return ErrForbidden
}

func targetWithDefaults(target Target, defaults Target) Target {
	if target.Resource == "" {
		target.Resource = defaults.Resource
	}
	if target.Action == "" {
		target.Action = defaults.Action
	}
	if target.Method == "" {
		target.Method = defaults.Method
	}
	if target.Path == "" {
		target.Path = defaults.Path
	}
	if len(defaults.Metadata) > 0 {
		if target.Metadata == nil {
			target.Metadata = make(map[string]string, len(defaults.Metadata))
		}
		for k, v := range defaults.Metadata {
			if _, ok := target.Metadata[k]; !ok {
				target.Metadata[k] = v
			}
		}
	}
	return target
}
