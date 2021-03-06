function bleveQuery(scope) {
    var from = (scope.page-1) * scope.resultsPerPage;
    return { "query": {
        "indexName": scope.indexName,
        "size": scope.resultsPerPage,
        "from": from,
        "explain": true,
        "highlight": {},
        "query": {
            "boost": 1.0,
            "query": scope.query,
        },
        "fields": ["*"],
    } }
}

function vliteQuery(scope) {
    return { "key": scope.query };
}

var prepQueryRequest = {
    "bleve": bleveQuery,
    "bleve-mem": bleveQuery,
    "alias": bleveQuery, // TODO: make aliases more generic.
    "vlite": vliteQuery,
    "vlite-mem": vliteQuery
};

function QueryCtrl($scope, $http, $routeParams, $log, $sce, $location) {

    $scope.maxPagesToShow = 5;
    $scope.resultsPerPage = 10;
    $scope.page = 1;
    $scope.timeout = 0;
    $scope.consistencyLevel = "";
    $scope.consistencyVectors = "{}";

    $scope.runQuery = function() {
        $scope.results = null;
        $scope.numPages = 0;

        $location.search('q', $scope.query);
        $location.search('p', $scope.page);

        var indexDefType = ($scope.indexDef && $scope.indexDef.type) || "bleve";

        var req = prepQueryRequest[indexDefType]($scope);
        req.consistency = {
            "level": $scope.consistencyLevel,
            "vectors": JSON.parse($scope.consistencyVectors || "null"),
        }
        req.timeout = parseInt($scope.timeout) || 0

        $http.post('/api/index/' + $scope.indexName + '/query', req).
        success(function(data) {
            $scope.processResults(data);
        }).
        error(function(data, code) {
            $scope.errorMessage = data;
        });
    };

    if($location.search().p !== undefined) {
        var page = parseInt($location.search().p, 10);
        if (typeof page == 'number' && !isNaN(page) && isFinite(page) && page > 0 ){
            $scope.page = page;
        }
    }

    if($location.search().q !== undefined) {
        $scope.query = $location.search().q;
        $scope.runQuery();
    }

    $scope.expl = function(explanation) {
        var rv = "" + $scope.roundScore(explanation.value) +
            " - " + explanation.message;
        rv = rv + "<ul>";
        for(var i in explanation.children) {
            var child = explanation.children[i];
            rv = rv + "<li>" + $scope.expl(child) + "</li>";
        }
        rv = rv + "</ul>";
        return rv;
    };

    $scope.roundScore = function(score) {
        return Math.round(score*1000)/1000;
    };

    $scope.roundTook = function(took) {
        if (!took || took <= 0) {
            return "";
        }
        if (took < 1000 * 1000) {
            return "<1ms";
        } else if (took < 1000 * 1000 * 1000) {
            return "" + Math.round(took / (1000*1000)) + "ms";
        } else {
            roundMs = Math.round(took / (1000*1000));
            return "" + roundMs/1000 + "s";
        }
	};

    $scope.setupPager = function(results) {
        if (!results.total_hits) {
            return;
        }

        $scope.numPages = Math.ceil(results.total_hits/$scope.resultsPerPage);
        $scope.validPages = [];
        for(var i = 1; i <= $scope.numPages; i++) {
            $scope.validPages.push(i);
        }

        // now see if we have too many pages
        if ($scope.validPages.length > $scope.maxPagesToShow) {
            var numPagesToRemove = $scope.validPages.length - $scope.maxPagesToShow;
            var frontPagesToRemove = 0
            var backPagesToRemove = 0;
            while (numPagesToRemove - frontPagesToRemove - backPagesToRemove > 0) {
                var numPagesBefore = $scope.page - 1 - frontPagesToRemove;
                var numPagesAfter =
                    $scope.validPages.length - $scope.page - backPagesToRemove;
                if (numPagesAfter > numPagesBefore) {
                    backPagesToRemove++;
                } else {
                    frontPagesToRemove++;
                }
            }

            // remove from the end first, to keep indexes simpler
            $scope.validPages.splice(-backPagesToRemove, backPagesToRemove);
            $scope.validPages.splice(0, frontPagesToRemove);
        }
        $scope.firstResult = (($scope.page-1) * $scope.resultsPerPage) + 1;
    };

    $scope.processResults = function(data) {
        $scope.errorMessage = null;
        $scope.results = data;
        $scope.setupPager($scope.results);
        for(var i in $scope.results.hits) {
            var hit = $scope.results.hits[i];
            hit.roundedScore = $scope.roundScore(hit.score);
            hit.explanationString = $scope.expl(hit.explanation);
            hit.explanationStringSafe = $sce.trustAsHtml(hit.explanationString);
            for(var ff in hit.fragments) {
                var fragments = hit.fragments[ff];
                var newFragments = [];
                for(var ffi in fragments) {
                    var fragment = fragments[ffi];
                    var safeFragment = $sce.trustAsHtml(fragment);
                    newFragments.push(safeFragment);
                }
                hit.fragments[ff] = newFragments;
            }
            if (!hit.fragments) {
                hit.fragments = {};
            }
            for(var fv in hit.fields) {
                var fieldval = hit.fields[fv];
                if (hit.fragments[fv] === undefined) {
                    hit.fragments[fv] = [$sce.trustAsHtml(""+fieldval)];
                }
            }
        }
        if (data.took) {
            $scope.results.roundTook = $scope.roundTook(data.took);
        }
    };

    $scope.jumpToPage = function(pageNum, $event) {
        if ($event) {
            $event.preventDefault();
        }

        $scope.page = pageNum;
        $scope.runQuery();
    };

}
