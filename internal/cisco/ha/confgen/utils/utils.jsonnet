{
    local utils = self,
    
    mergePatch(target, patch)::
        if std.isObject(patch) then
        local target_object =
            if std.isObject(target) then target else {};

        local target_fields =
            if std.isObject(target_object) then std.objectFields(target_object) else [];

        local null_fields = [k for k in std.objectFields(patch) if patch[k] == null];
        local both_fields = std.setUnion(target_fields, std.objectFields(patch));

        {
            [k]:
            if !std.objectHas(patch, k) then
                target_object[k]
            else if !std.objectHas(target_object, k) then
                utils.mergePatch(null, patch[k]) tailstrict
            else
                utils.mergePatch(target_object[k], patch[k]) tailstrict
            for k in std.setDiff(both_fields, null_fields)
        }
        else if std.isArray(patch) && std.isArray(target) then 
        target + patch
        else
        patch
}